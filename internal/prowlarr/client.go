package prowlarr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type IndexerSchema struct {
	ID                 int            `json:"id"`
	Name               string         `json:"name"`
	Implementation     string         `json:"implementation"`
	ImplementationName string         `json:"implementationName"`
	ConfigContract     string         `json:"configContract"`
	Tags               []string       `json:"tags"`
	Fields             []SchemaField  `json:"fields"`
}

type SchemaField struct {
	Name  string      `json:"name"`
	Label string      `json:"label"`
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"`
}

type Indexer struct {
	ID                 int            `json:"id"`
	Name               string         `json:"name"`
	Enable             bool           `json:"enable"`
	Implementation     string         `json:"implementation"`
	ImplementationName string         `json:"implementationName"`
	ConfigContract     string         `json:"configContract"`
	Fields             []SchemaField  `json:"fields"`
	Tags               []int          `json:"tags"`
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) Ping() error {
	resp, err := c.do("GET", "/api/v1/system/status", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("prowlarr returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetUnit3dSchemas() ([]IndexerSchema, error) {
	resp, err := c.do("GET", "/api/v1/indexer/schema", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("prowlarr returned HTTP %d", resp.StatusCode)
	}

	var schemas []IndexerSchema
	if err := json.Unmarshal(body, &schemas); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var out []IndexerSchema
	for _, s := range schemas {
		if isUnit3d(s) {
			out = append(out, s)
		}
	}
	return out, nil
}

func isUnit3d(s IndexerSchema) bool {
	impl := strings.ToLower(s.ImplementationName)
	if strings.Contains(impl, "unit3d") {
		return true
	}
	for _, tag := range s.Tags {
		if strings.EqualFold(tag, "unit3d") || strings.EqualFold(tag, "unit3d-community-edition") {
			return true
		}
	}
	return false
}

func (c *Client) GetIndexers() ([]Indexer, error) {
	resp, err := c.do("GET", "/api/v1/indexer", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []Indexer
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) SchemaByName(name string) (*IndexerSchema, error) {
	schemas, err := c.GetUnit3dSchemas()
	if err != nil {
		return nil, err
	}
	for _, s := range schemas {
		if strings.EqualFold(s.Name, name) {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("schema %q not found in Prowlarr", name)
}

func (c *Client) AddIndexer(schema IndexerSchema, trackerURL, apiKey string) (*Indexer, error) {
	fields := populateFields(schema.Fields, trackerURL, apiKey)

	payload := map[string]interface{}{
		"name":               schema.Name,
		"enable":             true,
		"implementation":     schema.Implementation,
		"implementationName": schema.ImplementationName,
		"configContract":     schema.ConfigContract,
		"fields":             fields,
		"tags":               []int{},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.do("POST", "/api/v1/indexer", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("prowlarr HTTP %d: %s", resp.StatusCode, string(body))
	}

	var idx Indexer
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func (c *Client) SetEnabled(indexer Indexer, enabled bool) error {
	indexer.Enable = enabled
	data, err := json.Marshal(indexer)
	if err != nil {
		return err
	}

	resp, err := c.do("PUT", fmt.Sprintf("/api/v1/indexer/%d", indexer.ID), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("prowlarr HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) DeleteIndexer(id int) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/api/v1/indexer/%d", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("prowlarr HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetIndexer(id int) (*Indexer, error) {
	resp, err := c.do("GET", fmt.Sprintf("/api/v1/indexer/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var idx Indexer
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func populateFields(fields []SchemaField, trackerURL, apiKey string) []SchemaField {
	out := make([]SchemaField, len(fields))
	copy(out, fields)

	for i, f := range out {
		low := strings.ToLower(f.Name)
		switch {
		case low == "baseurl" || low == "sitelink" || strings.Contains(low, "url"):
			out[i].Value = strings.TrimRight(trackerURL, "/")
		case low == "apikey" || low == "api_key" || low == "passkey" || low == "apitoken" || strings.Contains(low, "key") || strings.Contains(low, "token"):
			out[i].Value = apiKey
		}
	}
	return out
}

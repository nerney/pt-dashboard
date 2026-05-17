package unit3d

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type Stats struct {
	UserID   int
	Username string
	Upload   int64
	Download int64
	Ratio    float64
	Buffer   int64
	Bonus    float64
	Seeding  int
	Leeching int
	Class    string
}

// userListResponse matches UNIT3D's paginated user list
type userListResponse struct {
	Data []struct {
		ID         int `json:"id"`
		Attributes struct {
			Username string `json:"username"`
		} `json:"attributes"`
	} `json:"data"`
}

// userResponse matches UNIT3D's single user response
type userResponse struct {
	Data struct {
		ID         int `json:"id"`
		Attributes struct {
			Username    string  `json:"username"`
			Uploaded    int64   `json:"uploaded"`
			Downloaded  int64   `json:"downloaded"`
			Ratio       float64 `json:"ratio"`
			Buffer      int64   `json:"buffer"`
			BonusPoints float64 `json:"bonus_points"`
			Seeding     int     `json:"seeding"`
			Leeching    int     `json:"leeching"`
			Class       string  `json:"class"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *Client) FetchStats(username string) (*Stats, error) {
	id, err := c.resolveUserID(username)
	if err != nil {
		return nil, fmt.Errorf("resolve user: %w", err)
	}
	return c.fetchByID(id)
}

func (c *Client) resolveUserID(username string) (int, error) {
	endpoint := fmt.Sprintf("%s/api/users?filter[username]=%s", c.baseURL, url.QueryEscape(username))
	body, status, err := c.get(endpoint)
	if err != nil {
		return 0, err
	}
	if status != 200 {
		return 0, fmt.Errorf("HTTP %d from tracker", status)
	}

	var resp userListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parse user list: %w", err)
	}

	for _, u := range resp.Data {
		if strings.EqualFold(u.Attributes.Username, username) {
			return u.ID, nil
		}
	}
	if len(resp.Data) > 0 {
		return resp.Data[0].ID, nil
	}
	return 0, fmt.Errorf("user %q not found", username)
}

func (c *Client) fetchByID(id int) (*Stats, error) {
	endpoint := fmt.Sprintf("%s/api/users/%d", c.baseURL, id)
	body, status, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("HTTP %d from tracker", status)
	}

	var resp userResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	a := resp.Data.Attributes
	buf := a.Buffer
	if buf == 0 && a.Uploaded > a.Downloaded {
		buf = a.Uploaded - a.Downloaded
	}

	return &Stats{
		UserID:   resp.Data.ID,
		Username: a.Username,
		Upload:   a.Uploaded,
		Download: a.Downloaded,
		Ratio:    a.Ratio,
		Buffer:   buf,
		Bonus:    a.BonusPoints,
		Seeding:  a.Seeding,
		Leeching: a.Leeching,
		Class:    a.Class,
	}, nil
}

func (c *Client) get(endpoint string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (c *Client) Ping(username string) error {
	_, err := c.resolveUserID(username)
	return err
}

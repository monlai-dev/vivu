package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type MatrixPoint struct {
	ID  string
	Lat float64
	Lng float64
}

type MatrixEdge struct {
	DistanceMeters int
}

type DistanceMatrix map[string]map[string]MatrixEdge

// --------- In-memory cache theo cặp (A,B) ---------

type pairKey struct {
	Mode string // "driving"
	A    string // ID POI ổn định
	B    string
}

type matrixPairCacheEntry struct {
	Edge      MatrixEdge
	ExpiresAt time.Time
}

type MatrixPairCache interface {
	Get(k pairKey) (MatrixEdge, bool)
	Set(k pairKey, v MatrixEdge, ttl time.Duration)
}

type inMemoryPairCache struct {
	mu    sync.RWMutex
	store map[pairKey]matrixPairCacheEntry
}

func NewInMemoryPairCache() MatrixPairCache {
	return &inMemoryPairCache{store: make(map[pairKey]matrixPairCacheEntry)}
}

func (c *inMemoryPairCache) Get(k pairKey) (MatrixEdge, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.store[k]
	if !ok || time.Now().After(it.ExpiresAt) {
		return MatrixEdge{}, false
	}
	return it.Edge, true
}

func (c *inMemoryPairCache) Set(k pairKey, v MatrixEdge, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[k] = matrixPairCacheEntry{Edge: v, ExpiresAt: time.Now().Add(ttl)}
}

// -------------- Mapbox Matrix client (distance-only) ---------------

type DistanceMatrixService interface {
	ComputeDistances(ctx context.Context, points []MatrixPoint) (DistanceMatrix, error)
}

type MapboxMatrixClient struct {
	HTTP        *http.Client
	AccessToken string
	Cache       MatrixPairCache
	DefaultTTL  time.Duration // ví dụ 7 ngày
	Profile     string        // "driving"
}

func NewMapboxMatrixClient(cache MatrixPairCache) *MapboxMatrixClient {
	token := os.Getenv("MAPBOX_ACCESS_TOKEN")
	if token == "" {
		panic("MAPBOX_ACCESS_TOKEN is empty")
	}
	return &MapboxMatrixClient{
		HTTP:        &http.Client{Timeout: 15 * time.Second},
		AccessToken: token,
		Cache:       cache,
		DefaultTTL:  7 * 24 * time.Hour,
		Profile:     "driving",
	}
}

func (c *MapboxMatrixClient) ComputeDistances(ctx context.Context, points []MatrixPoint) (DistanceMatrix, error) {
	n := len(points)
	if n == 0 {
		return DistanceMatrix{}, nil
	}

	mode := c.Profile
	mat := make(DistanceMatrix, n)
	needCall := false

	for _, p := range points {
		mat[p.ID] = make(map[string]MatrixEdge, n)
	}

	// 1) Thử lấy từ cache
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				mat[points[i].ID][points[j].ID] = MatrixEdge{DistanceMeters: 0}
				continue
			}
			k := pairKey{Mode: mode, A: points[i].ID, B: points[j].ID}
			if v, ok := c.Cache.Get(k); ok {
				mat[points[i].ID][points[j].ID] = v
			} else {
				needCall = true
			}
		}
	}

	if !needCall {
		return mat, nil
	}

	// 2) Gọi Mapbox Matrix cho toàn bộ tập điểm
	coords := make([]string, 0, n)
	for _, p := range points {
		coords = append(coords, fmt.Sprintf("%f,%f", p.Lng, p.Lat))
	}
	coordStr := strings.Join(coords, ";")

	u := url.URL{
		Scheme: "https",
		Host:   "api.mapbox.com",
		Path:   fmt.Sprintf("/directions-matrix/v1/mapbox/%s/%s", mode, coordStr),
	}
	q := url.Values{}
	q.Set("annotations", "distance") // chỉ cần distance
	q.Set("sources", "all")
	q.Set("destinations", "all")
	q.Set("access_token", c.AccessToken)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mapbox matrix http error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("mapbox matrix bad status: %s", resp.Status)
	}

	var payload struct {
		Distances [][]*float64 `json:"distances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("mapbox decode: %w", err)
	}

	// 3) Ghi vào matrix + cache
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				mat[points[i].ID][points[j].ID] = MatrixEdge{DistanceMeters: 0}
				continue
			}
			dM := 0
			if payload.Distances != nil && i < len(payload.Distances) && j < len(payload.Distances[i]) && payload.Distances[i][j] != nil {
				dM = int(*payload.Distances[i][j] + 0.5)
			}
			edge := MatrixEdge{DistanceMeters: dM}
			mat[points[i].ID][points[j].ID] = edge
			c.Cache.Set(pairKey{Mode: mode, A: points[i].ID, B: points[j].ID}, edge, c.DefaultTTL)
		}
	}

	return mat, nil
}

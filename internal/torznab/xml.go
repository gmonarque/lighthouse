package torznab

import (
	"encoding/xml"
	"fmt"
	"time"
)

// RSS represents the root RSS element
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Atom    string   `xml:"xmlns:atom,attr"`
	Torznab string   `xml:"xmlns:torznab,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel represents the RSS channel
type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Language    string `xml:"language"`
	Category    string `xml:"category"`
	Items       []Item `xml:"item"`

	// For caps response
	Response *Response `xml:"response,omitempty"`
}

// Item represents a single torrent result
type Item struct {
	Title       string     `xml:"title"`
	GUID        string     `xml:"guid"`
	Link        string     `xml:"link"`
	Comments    string     `xml:"comments,omitempty"`
	PubDate     string     `xml:"pubDate"`
	Size        int64      `xml:"size"`
	Description string     `xml:"description,omitempty"`
	Category    string     `xml:"category"`
	Enclosure   *Enclosure `xml:"enclosure,omitempty"`
	Attributes  []Attr     `xml:"torznab:attr"`
}

// Enclosure represents the torrent file/magnet link
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// Attr represents a Torznab attribute
type Attr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Response for caps endpoint
type Response struct {
	Offset int `xml:"offset,attr"`
	Total  int `xml:"total,attr"`
}

// Caps represents the capabilities response
type Caps struct {
	XMLName    xml.Name       `xml:"caps"`
	Server     CapsServer     `xml:"server"`
	Limits     CapsLimits     `xml:"limits"`
	Searching  CapsSearching  `xml:"searching"`
	Categories CapsCategories `xml:"categories"`
}

// CapsServer represents server info in caps
type CapsServer struct {
	Version   string `xml:"version,attr"`
	Title     string `xml:"title,attr"`
	Strapline string `xml:"strapline,attr"`
	Email     string `xml:"email,attr,omitempty"`
	URL       string `xml:"url,attr,omitempty"`
	Image     string `xml:"image,attr,omitempty"`
}

// CapsLimits represents limits in caps
type CapsLimits struct {
	Max     int `xml:"max,attr"`
	Default int `xml:"default,attr"`
}

// CapsSearching represents search capabilities
type CapsSearching struct {
	Search      CapsSearch `xml:"search"`
	TVSearch    CapsSearch `xml:"tv-search"`
	MovieSearch CapsSearch `xml:"movie-search"`
	MusicSearch CapsSearch `xml:"music-search"`
	BookSearch  CapsSearch `xml:"book-search"`
}

// CapsSearch represents a single search type capability
type CapsSearch struct {
	Available       string `xml:"available,attr"`
	SupportedParams string `xml:"supportedParams,attr"`
}

// CapsCategories represents available categories
type CapsCategories struct {
	Categories []CapsCategory `xml:"category"`
}

// CapsCategory represents a category in caps
type CapsCategory struct {
	ID     int              `xml:"id,attr"`
	Name   string           `xml:"name,attr"`
	SubCat []CapsSubCategory `xml:"subcat,omitempty"`
}

// CapsSubCategory represents a subcategory
type CapsSubCategory struct {
	ID   int    `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

// NewCaps creates a capabilities response
func NewCaps(baseURL string) *Caps {
	cats := AllCategories()
	capsCats := make([]CapsCategory, len(cats))

	for i, cat := range cats {
		subCats := make([]CapsSubCategory, len(cat.SubCats))
		for j, sub := range cat.SubCats {
			subCats[j] = CapsSubCategory{
				ID:   sub.ID,
				Name: sub.Name,
			}
		}
		capsCats[i] = CapsCategory{
			ID:     cat.ID,
			Name:   cat.Name,
			SubCat: subCats,
		}
	}

	return &Caps{
		Server: CapsServer{
			Version:   "1.0",
			Title:     "Lighthouse",
			Strapline: "Decentralized Torrent Indexer",
			URL:       baseURL,
		},
		Limits: CapsLimits{
			Max:     100,
			Default: 50,
		},
		Searching: CapsSearching{
			Search: CapsSearch{
				Available:       "yes",
				SupportedParams: "q",
			},
			TVSearch: CapsSearch{
				Available:       "yes",
				SupportedParams: "q,season,ep",
			},
			MovieSearch: CapsSearch{
				Available:       "yes",
				SupportedParams: "q,imdbid,tmdbid",
			},
			MusicSearch: CapsSearch{
				Available:       "yes",
				SupportedParams: "q,artist,album",
			},
			BookSearch: CapsSearch{
				Available:       "yes",
				SupportedParams: "q,author,title",
			},
		},
		Categories: CapsCategories{
			Categories: capsCats,
		},
	}
}

// NewSearchResponse creates a search results response
func NewSearchResponse(baseURL string, results []SearchResult, offset, total int) *RSS {
	items := make([]Item, len(results))

	for i, r := range results {
		pubDate := r.PubDate
		if pubDate.IsZero() {
			pubDate = time.Now()
		}

		items[i] = Item{
			Title:       r.Title,
			GUID:        r.GUID,
			Link:        r.MagnetURI,
			PubDate:     pubDate.Format(time.RFC1123Z),
			Size:        r.Size,
			Description: r.Description,
			Category:    fmt.Sprintf("%d", r.Category),
			Enclosure: &Enclosure{
				URL:    r.MagnetURI,
				Length: r.Size,
				Type:   "application/x-bittorrent;x-scheme-handler/magnet",
			},
			Attributes: []Attr{
				{Name: "category", Value: fmt.Sprintf("%d", r.Category)},
				{Name: "size", Value: fmt.Sprintf("%d", r.Size)},
				{Name: "seeders", Value: fmt.Sprintf("%d", r.Seeders)},
				{Name: "leechers", Value: fmt.Sprintf("%d", r.Leechers)},
				{Name: "infohash", Value: r.InfoHash},
				{Name: "magneturl", Value: r.MagnetURI},
			},
		}

		// Add optional attributes
		if r.ImdbID != "" {
			items[i].Attributes = append(items[i].Attributes, Attr{Name: "imdbid", Value: r.ImdbID})
		}
		if r.TmdbID > 0 {
			items[i].Attributes = append(items[i].Attributes, Attr{Name: "tmdbid", Value: fmt.Sprintf("%d", r.TmdbID)})
		}
		if r.Year > 0 {
			items[i].Attributes = append(items[i].Attributes, Attr{Name: "year", Value: fmt.Sprintf("%d", r.Year)})
		}
		if r.PosterURL != "" {
			items[i].Attributes = append(items[i].Attributes, Attr{Name: "coverurl", Value: r.PosterURL})
		}
	}

	return &RSS{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Torznab: "http://torznab.com/schemas/2015/feed",
		Channel: Channel{
			Title:       "Lighthouse",
			Description: "Decentralized Torrent Indexer",
			Link:        baseURL,
			Language:    "en-us",
			Category:    "",
			Items:       items,
			Response: &Response{
				Offset: offset,
				Total:  total,
			},
		},
	}
}

// SearchResult represents a torrent search result
type SearchResult struct {
	Title       string
	GUID        string
	InfoHash    string
	MagnetURI   string
	Size        int64
	Category    int
	Seeders     int
	Leechers    int
	PubDate     time.Time
	Description string
	ImdbID      string
	TmdbID      int
	Year        int
	PosterURL   string
}

// ErrorResponse creates an error response
type ErrorResponse struct {
	XMLName     xml.Name `xml:"error"`
	Code        int      `xml:"code,attr"`
	Description string   `xml:"description,attr"`
}

// NewErrorResponse creates an error response
func NewErrorResponse(code int, description string) *ErrorResponse {
	return &ErrorResponse{
		Code:        code,
		Description: description,
	}
}

// Common error codes
const (
	ErrorIncorrectUserCreds   = 100
	ErrorAccountSuspended     = 101
	ErrorInsufficientPrivs    = 102
	ErrorRegistrationDenied   = 103
	ErrorRegistrationClosed   = 104
	ErrorEmailAlreadyExists   = 105
	ErrorInvalidIMDBID        = 200
	ErrorTorrentNotFound      = 201
	ErrorRequestLimitReached  = 500
	ErrorNoFunction           = 900
	ErrorNoParameter          = 901
	ErrorNoResults            = 902
)

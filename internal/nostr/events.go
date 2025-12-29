package nostr

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

// TorrentEvent represents a parsed torrent event from Nostr
type TorrentEvent struct {
	EventID     string
	Pubkey      string
	CreatedAt   int64
	MagnetURI   string
	InfoHash    string
	Name        string
	Size        int64
	Category    string
	Files       []TorrentFile
	Tags        map[string]string
	ContentTags []string // t tags for content classification (movie, tv, 4k, hd, etc.)
	Description string   // Event content if not a magnet URI, or from summary tag
}

// TorrentFile represents a file in a torrent
type TorrentFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ParseTorrentEvent parses a Kind 2003 Nostr event into a TorrentEvent
func ParseTorrentEvent(event *nostr.Event) (*TorrentEvent, error) {
	if event.Kind != KindTorrent {
		return nil, nil
	}

	te := &TorrentEvent{
		EventID:     event.ID,
		Pubkey:      event.PubKey,
		CreatedAt:   int64(event.CreatedAt),
		Tags:        make(map[string]string),
		ContentTags: make([]string, 0),
		Files:       make([]TorrentFile, 0),
	}

	// Check if content is a magnet URI or description
	content := strings.TrimSpace(event.Content)
	if strings.HasPrefix(strings.ToLower(content), "magnet:") {
		te.MagnetURI = content
	} else if content != "" {
		te.Description = content
	}

	// Parse tags
	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}

		key := tag[0]
		value := tag[1]

		switch key {
		case "title", "name":
			te.Name = value
		case "x", "btih", "infohash":
			te.InfoHash = strings.ToLower(value)
		case "size":
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				te.Size = size
			}
		case "category", "cat":
			te.Category = value
		case "t":
			// Content tags (movie, tv, 4k, hd, etc.)
			te.ContentTags = append(te.ContentTags, strings.ToLower(value))
		case "file":
			// NIP-35 file tag: ["file", "filename", "size"]
			file := TorrentFile{Name: value}
			if len(tag) >= 3 {
				if size, err := strconv.ParseInt(tag[2], 10, 64); err == nil {
					file.Size = size
				}
			}
			te.Files = append(te.Files, file)
		case "summary":
			// Some events use summary tag for description
			if te.Description == "" {
				te.Description = value
			}
		default:
			te.Tags[key] = value
		}
	}

	// Extract info hash from magnet URI if not in tags
	if te.InfoHash == "" && te.MagnetURI != "" {
		te.InfoHash = extractInfoHash(te.MagnetURI)
	}

	// Extract name from magnet URI if not in tags
	if te.Name == "" && te.MagnetURI != "" {
		te.Name = extractNameFromMagnet(te.MagnetURI)
	}

	// Calculate total size from files if not explicitly set
	// (do this before building magnet URI so we can include size)
	if te.Size == 0 && len(te.Files) > 0 {
		for _, f := range te.Files {
			te.Size += f.Size
		}
	}

	// Generate magnet URI if we have info hash but no magnet URI
	// Per https://en.wikipedia.org/wiki/Magnet_URI_scheme
	if te.MagnetURI == "" && te.InfoHash != "" {
		te.MagnetURI = buildMagnetURI(te.InfoHash, te.Name, te.Size)
	}

	return te, nil
}

// ParseContactList parses a Kind 3 Nostr event (contact list)
func ParseContactList(event *nostr.Event) []string {
	if event.Kind != KindContactList {
		return nil
	}

	var contacts []string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			contacts = append(contacts, tag[1])
		}
	}

	return contacts
}

// extractInfoHash extracts the info hash from a magnet URI
func extractInfoHash(magnetURI string) string {
	// Match btih (BitTorrent Info Hash) in magnet URI
	re := regexp.MustCompile(`(?i)btih:([a-fA-F0-9]{40}|[a-zA-Z2-7]{32})`)
	matches := re.FindStringSubmatch(magnetURI)
	if len(matches) > 1 {
		hash := strings.ToLower(matches[1])
		// Convert base32 to hex if needed
		if len(hash) == 32 {
			// This is base32 encoded, would need conversion
			// For now, return as-is
			return hash
		}
		return hash
	}
	return ""
}

// extractNameFromMagnet extracts the display name from a magnet URI
func extractNameFromMagnet(magnetURI string) string {
	re := regexp.MustCompile(`dn=([^&]+)`)
	matches := re.FindStringSubmatch(magnetURI)
	if len(matches) > 1 {
		name := matches[1]
		// URL decode
		name = strings.ReplaceAll(name, "+", " ")
		name = strings.ReplaceAll(name, "%20", " ")
		name = strings.ReplaceAll(name, ".", " ")
		return strings.TrimSpace(name)
	}
	return ""
}

// buildMagnetURI constructs a magnet URI from info hash, name, and size
// Per https://en.wikipedia.org/wiki/Magnet_URI_scheme
func buildMagnetURI(infoHash, name string, size int64) string {
	// Start with the required xt (exact topic) parameter
	magnet := "magnet:?xt=urn:btih:" + strings.ToLower(infoHash)

	// Add display name (dn) if available - URL encode it
	if name != "" {
		// Simple URL encoding for the name
		encodedName := strings.ReplaceAll(name, " ", "+")
		encodedName = strings.ReplaceAll(encodedName, "&", "%26")
		encodedName = strings.ReplaceAll(encodedName, "=", "%3D")
		encodedName = strings.ReplaceAll(encodedName, "#", "%23")
		magnet += "&dn=" + encodedName
	}

	// Add exact length (xl) if available
	if size > 0 {
		magnet += "&xl=" + strconv.FormatInt(size, 10)
	}

	// Add common public trackers for better connectivity
	trackers := []string{
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://open.stealth.si:80/announce",
		"udp://tracker.torrent.eu.org:451/announce",
		"udp://tracker.bittor.pw:1337/announce",
		"udp://public.popcorn-tracker.org:6969/announce",
		"udp://tracker.dler.org:6969/announce",
		"udp://exodus.desync.com:6969",
		"udp://open.demonii.com:1337/announce",
	}

	for _, tr := range trackers {
		// URL encode the tracker
		encoded := strings.ReplaceAll(tr, ":", "%3A")
		encoded = strings.ReplaceAll(encoded, "/", "%2F")
		magnet += "&tr=" + encoded
	}

	return magnet
}

// CreateTorrentEvent creates a new Kind 2003 torrent event
func CreateTorrentEvent(magnetURI, name, category string, size int64, infoHash string) *nostr.Event {
	tags := nostr.Tags{
		{"x", infoHash},
		{"title", name},
		{"size", strconv.FormatInt(size, 10)},
	}

	if category != "" {
		tags = append(tags, nostr.Tag{"category", category})
	}

	return &nostr.Event{
		Kind:      KindTorrent,
		Content:   magnetURI,
		Tags:      tags,
		CreatedAt: nostr.Now(),
	}
}

// CategoryFromNostrTags determines Torznab category from a list of NIP-35 tags
// Tags follow hierarchical structure: video->movie->4k, audio->music->flac, etc.
func CategoryFromNostrTags(tags []string) int {
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	// Check for adult content first (highest priority)
	if tagSet["xxx"] || tagSet["adult"] || tagSet["porn"] || tagSet["nsfw"] {
		return 6000 // XXX
	}

	// Video categories (Movies & TV)
	if tagSet["video"] || tagSet["movie"] || tagSet["movies"] || tagSet["film"] {
		// Check for quality subcategories
		if tagSet["4k"] || tagSet["uhd"] || tagSet["2160p"] {
			return 2045 // Movies/UHD
		}
		if tagSet["hd"] || tagSet["1080p"] || tagSet["720p"] || tagSet["bluray"] || tagSet["blu-ray"] {
			return 2040 // Movies/HD
		}
		if tagSet["dvdr"] || tagSet["dvd"] {
			return 2070 // Movies/DVD
		}
		if tagSet["web-dl"] || tagSet["webdl"] || tagSet["webrip"] {
			return 2080 // Movies/WEB-DL
		}
		return 2000 // Movies (general)
	}

	if tagSet["tv"] || tagSet["series"] || tagSet["show"] || tagSet["television"] {
		if tagSet["4k"] || tagSet["uhd"] || tagSet["2160p"] {
			return 5045 // TV/UHD
		}
		if tagSet["hd"] || tagSet["1080p"] || tagSet["720p"] {
			return 5040 // TV/HD
		}
		if tagSet["anime"] {
			return 5070 // TV/Anime
		}
		if tagSet["documentary"] || tagSet["doc"] {
			return 5080 // TV/Documentary
		}
		if tagSet["sport"] || tagSet["sports"] {
			return 5060 // TV/Sport
		}
		return 5000 // TV (general)
	}

	// Anime (can be standalone)
	if tagSet["anime"] {
		return 5070 // TV/Anime
	}

	// Audio categories
	if tagSet["audio"] || tagSet["music"] || tagSet["soundtrack"] || tagSet["ost"] {
		if tagSet["flac"] || tagSet["lossless"] || tagSet["alac"] {
			return 3040 // Audio/Lossless
		}
		if tagSet["audio-book"] || tagSet["audiobook"] {
			return 3030 // Audio/Audiobook
		}
		if tagSet["mp3"] {
			return 3010 // Audio/MP3
		}
		return 3000 // Audio (general)
	}

	// Audiobooks (standalone)
	if tagSet["audio-book"] || tagSet["audiobook"] {
		return 3030 // Audio/Audiobook
	}

	// Applications/Software
	if tagSet["application"] || tagSet["software"] || tagSet["app"] || tagSet["apps"] {
		if tagSet["windows"] || tagSet["win"] {
			return 4020 // PC/ISO (Windows)
		}
		if tagSet["mac"] || tagSet["macos"] || tagSet["osx"] {
			return 4030 // PC/Mac
		}
		if tagSet["linux"] || tagSet["unix"] {
			return 4020 // PC/ISO
		}
		if tagSet["ios"] || tagSet["iphone"] || tagSet["ipad"] {
			return 4060 // PC/Mobile-iOS
		}
		if tagSet["android"] || tagSet["apk"] {
			return 4070 // PC/Mobile-Android
		}
		return 4000 // PC (general)
	}

	// Games
	if tagSet["game"] || tagSet["games"] {
		if tagSet["pc"] || tagSet["windows"] {
			return 4050 // PC/Games
		}
		if tagSet["mac"] {
			return 4030 // PC/Mac
		}
		if tagSet["psx"] || tagSet["playstation"] || tagSet["ps3"] || tagSet["ps4"] || tagSet["ps5"] {
			return 1080 // Console/PS3 (generic PlayStation)
		}
		if tagSet["xbox"] || tagSet["xbox360"] || tagSet["xboxone"] {
			return 1040 // Console/Xbox
		}
		if tagSet["wii"] || tagSet["nintendo"] || tagSet["switch"] {
			return 1030 // Console/Wii
		}
		if tagSet["ios"] || tagSet["iphone"] {
			return 4060 // PC/Mobile-iOS
		}
		if tagSet["android"] {
			return 4070 // PC/Mobile-Android
		}
		return 4050 // PC/Games (default for games)
	}

	// Books/E-Books
	if tagSet["book"] || tagSet["books"] || tagSet["e-book"] || tagSet["ebook"] {
		if tagSet["comic"] || tagSet["comics"] || tagSet["manga"] {
			return 7030 // Books/Comics
		}
		if tagSet["magazine"] || tagSet["mag"] {
			return 7010 // Books/Mags
		}
		if tagSet["technical"] || tagSet["programming"] {
			return 7040 // Books/Technical
		}
		return 7020 // Books/EBook
	}

	// Comics (standalone)
	if tagSet["comic"] || tagSet["comics"] || tagSet["manga"] {
		return 7030 // Books/Comics
	}

	// Pictures
	if tagSet["picture"] || tagSet["pictures"] || tagSet["image"] || tagSet["images"] || tagSet["photo"] {
		return 8010 // Other/Misc
	}

	// Archives
	if tagSet["archive"] || tagSet["archives"] {
		return 8010 // Other/Misc
	}

	return 8000 // Other
}

// NostrTagFromCategory converts a Torznab category code to Nostr category tag
func NostrTagFromCategory(code int) string {
	// Get the base category (thousands)
	base := (code / 1000) * 1000

	categories := map[int]string{
		1000: "games",
		2000: "movie",
		3000: "music",
		4000: "software",
		5000: "tv",
		6000: "xxx",
		7000: "books",
		8000: "other",
	}

	if tag, ok := categories[base]; ok {
		return tag
	}

	return "other"
}

// PublishTorrentRequest contains all fields for publishing a torrent event
type PublishTorrentRequest struct {
	InfoHash    string        `json:"info_hash"`
	Name        string        `json:"name"`
	Size        int64         `json:"size"`
	Category    int           `json:"category"`
	Files       []TorrentFile `json:"files"`
	Trackers    []string      `json:"trackers"`
	Tags        []string      `json:"tags"`
	Description string        `json:"description"`
	ImdbID      string        `json:"imdb_id"`
	TmdbID      string        `json:"tmdb_id"`
}

// CreateFullTorrentEvent creates a Kind 2003 event with full NIP-35 support
func CreateFullTorrentEvent(req PublishTorrentRequest) *nostr.Event {
	tags := nostr.Tags{}

	// Required: info hash
	if req.InfoHash != "" {
		tags = append(tags, nostr.Tag{"x", strings.ToLower(req.InfoHash)})
	}

	// Required: title
	if req.Name != "" {
		tags = append(tags, nostr.Tag{"title", req.Name})
	}

	// Required: size
	if req.Size > 0 {
		tags = append(tags, nostr.Tag{"size", strconv.FormatInt(req.Size, 10)})
	}

	// Category as Nostr tag (movie, tv, etc.)
	if req.Category > 0 {
		categoryTag := NostrTagFromCategory(req.Category)
		tags = append(tags, nostr.Tag{"t", categoryTag})
	}

	// File entries (NIP-35 format)
	for _, file := range req.Files {
		tags = append(tags, nostr.Tag{"file", file.Name, strconv.FormatInt(file.Size, 10)})
	}

	// Trackers
	for _, tracker := range req.Trackers {
		if tracker != "" {
			tags = append(tags, nostr.Tag{"tracker", tracker})
		}
	}

	// Content tags (4k, hd, hdr, x265, etc.)
	for _, tag := range req.Tags {
		if tag != "" {
			tags = append(tags, nostr.Tag{"t", strings.ToLower(tag)})
		}
	}

	// External IDs
	if req.ImdbID != "" {
		// Ensure proper format
		imdb := req.ImdbID
		if !strings.HasPrefix(imdb, "tt") {
			imdb = "tt" + imdb
		}
		tags = append(tags, nostr.Tag{"i", "imdb:" + imdb})
	}

	if req.TmdbID != "" {
		tags = append(tags, nostr.Tag{"i", "tmdb:" + req.TmdbID})
	}

	// Alt tag for client preview
	if req.Name != "" {
		tags = append(tags, nostr.Tag{"alt", "Torrent: " + req.Name})
	}

	// NIP-35: content is description only (not magnet URI)
	content := req.Description

	return &nostr.Event{
		Kind:      KindTorrent,
		Content:   content,
		Tags:      tags,
		CreatedAt: nostr.Now(),
	}
}

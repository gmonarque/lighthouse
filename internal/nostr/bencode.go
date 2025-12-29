package nostr

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/jackpal/bencode-go"
)

// TorrentFileInfo contains parsed information from a .torrent file
type TorrentFileInfo struct {
	InfoHash string        `json:"info_hash"`
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	Files    []TorrentFile `json:"files"`
	Trackers []string      `json:"trackers"`
	Comment  string        `json:"comment"`
}

// torrentMeta represents the structure of a .torrent file
type torrentMeta struct {
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list"`
	Comment      string      `bencode:"comment"`
	Info         torrentInfo `bencode:"info"`
}

// torrentInfo represents the info dictionary in a torrent file
type torrentInfo struct {
	Name        string            `bencode:"name"`
	PieceLength int64             `bencode:"piece length"`
	Pieces      string            `bencode:"pieces"`
	Length      int64             `bencode:"length"`       // Single file mode
	Files       []torrentFileItem `bencode:"files"`        // Multi-file mode
}

// torrentFileItem represents a file in multi-file mode
type torrentFileItem struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

// ParseTorrentFile parses a .torrent file and extracts metadata
func ParseTorrentFile(data []byte) (*TorrentFileInfo, error) {
	reader := bytes.NewReader(data)

	var meta torrentMeta
	if err := bencode.Unmarshal(reader, &meta); err != nil {
		return nil, fmt.Errorf("failed to decode torrent: %w", err)
	}

	// Calculate info hash
	infoHash, err := calculateInfoHash(data)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate info hash: %w", err)
	}

	result := &TorrentFileInfo{
		InfoHash: infoHash,
		Name:     meta.Info.Name,
		Comment:  meta.Comment,
	}

	// Extract trackers
	if meta.Announce != "" {
		result.Trackers = append(result.Trackers, meta.Announce)
	}
	for _, tier := range meta.AnnounceList {
		for _, tracker := range tier {
			// Avoid duplicates
			found := false
			for _, existing := range result.Trackers {
				if existing == tracker {
					found = true
					break
				}
			}
			if !found {
				result.Trackers = append(result.Trackers, tracker)
			}
		}
	}

	// Extract files and calculate total size
	if len(meta.Info.Files) > 0 {
		// Multi-file mode
		for _, f := range meta.Info.Files {
			name := ""
			for i, part := range f.Path {
				if i > 0 {
					name += "/"
				}
				name += part
			}
			result.Files = append(result.Files, TorrentFile{
				Name: name,
				Size: f.Length,
			})
			result.Size += f.Length
		}
	} else {
		// Single file mode
		result.Files = []TorrentFile{{
			Name: meta.Info.Name,
			Size: meta.Info.Length,
		}}
		result.Size = meta.Info.Length
	}

	return result, nil
}

// calculateInfoHash extracts the info dictionary and calculates its SHA1 hash
// by finding the raw "info" dictionary bytes in the bencode data
func calculateInfoHash(data []byte) (string, error) {
	// Find "4:info" marker in the bencode data
	infoKey := []byte("4:infod")
	idx := bytes.Index(data, infoKey)
	if idx == -1 {
		return "", fmt.Errorf("torrent missing info dictionary")
	}

	// Start of info dictionary (the 'd' after "4:info")
	infoStart := idx + 6 // len("4:info") = 6

	// Find the matching 'e' that closes this dictionary
	// We need to track nesting depth
	depth := 0
	infoEnd := -1
	i := infoStart

	for i < len(data) {
		switch data[i] {
		case 'd', 'l': // dictionary or list start
			depth++
			i++
		case 'e': // end marker
			depth--
			if depth == 0 {
				infoEnd = i + 1
				break
			}
			i++
		case 'i': // integer: i<number>e
			i++
			for i < len(data) && data[i] != 'e' {
				i++
			}
			i++ // skip the 'e'
		default:
			// Must be a string: <length>:<content>
			// Parse the length
			lenStart := i
			for i < len(data) && data[i] >= '0' && data[i] <= '9' {
				i++
			}
			if i >= len(data) || data[i] != ':' {
				return "", fmt.Errorf("invalid bencode: expected ':' after string length")
			}
			lenBytes := data[lenStart:i]
			var strLen int
			for _, b := range lenBytes {
				strLen = strLen*10 + int(b-'0')
			}
			i++ // skip ':'
			i += strLen // skip string content
		}
		if infoEnd != -1 {
			break
		}
	}

	if infoEnd == -1 {
		return "", fmt.Errorf("could not find end of info dictionary")
	}

	// Extract info dictionary bytes and hash them
	infoBytes := data[infoStart:infoEnd]
	hash := sha1.Sum(infoBytes)
	return hex.EncodeToString(hash[:]), nil
}

// ParseTorrentReader parses a torrent from an io.Reader
func ParseTorrentReader(r io.Reader) (*TorrentFileInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent data: %w", err)
	}
	return ParseTorrentFile(data)
}

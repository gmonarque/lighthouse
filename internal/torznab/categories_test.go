package torznab

import "testing"

func TestMapNostrCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"movie", CategoryMovies},
		{"movies", CategoryMovies},
		{"film", CategoryMovies},
		{"tv", CategoryTV},
		{"series", CategoryTV},
		{"music", CategoryAudio},
		{"games", CategoryPCGames},
		{"software", CategoryPC},
		{"books", CategoryBooks},
		{"ebook", CategoryBooksEBook},
		{"xxx", CategoryXXX},
		{"unknown", CategoryOther},
		{"", CategoryOther},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapNostrCategory(tt.input)
			if result != tt.expected {
				t.Errorf("MapNostrCategory(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCategoryName(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{2000, "Movies"},
		{2040, "Movies"},
		{5000, "TV"},
		{5070, "TV"},
		{3000, "Audio"},
		{9999, "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := CategoryName(tt.input)
			if result != tt.expected {
				t.Errorf("CategoryName(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCategories(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"2000", []int{2000}},
		{"2000,5000", []int{2000, 5000}},
		{"2000, 5000, 3000", []int{2000, 5000, 3000}},
		{"", nil},
		{"invalid", []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseCategories(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseCategories(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("ParseCategories(%q)[%d] = %d, want %d", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizeImdbID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tt1234567", "tt1234567"},
		{"1234567", "tt1234567"},
		{"tt0111161", "tt0111161"},
		{"0111161", "tt0111161"},
		{"", ""},
		{"  tt1234567  ", "tt1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeImdbID(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeImdbID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

package torznab

// Torznab category codes
const (
	// Console
	CategoryConsole       = 1000
	CategoryConsoleNDS    = 1010
	CategoryConsolePSP    = 1020
	CategoryConsoleWii    = 1030
	CategoryConsoleXbox   = 1040
	CategoryConsoleXbox360= 1050
	CategoryConsoleWiiware= 1060
	CategoryConsoleXbox360DLC = 1070
	CategoryConsolePS3    = 1080
	CategoryConsoleOther  = 1090
	CategoryConsole3DS    = 1110
	CategoryConsolePSVita = 1120
	CategoryConsoleWiiU   = 1130
	CategoryConsoleXboxOne= 1140
	CategoryConsolePS4    = 1180

	// Movies
	CategoryMovies       = 2000
	CategoryMoviesForeign= 2010
	CategoryMoviesOther  = 2020
	CategoryMoviesSD     = 2030
	CategoryMoviesHD     = 2040
	CategoryMoviesUHD    = 2045
	CategoryMovies3D     = 2050
	CategoryMoviesBluRay = 2060
	CategoryMoviesDVD    = 2070
	CategoryMoviesWEBDL  = 2080

	// Audio
	CategoryAudio       = 3000
	CategoryAudioMP3    = 3010
	CategoryAudioVideo  = 3020
	CategoryAudioAudiobook = 3030
	CategoryAudioLossless = 3040
	CategoryAudioOther  = 3050
	CategoryAudioForeign= 3060

	// PC
	CategoryPC         = 4000
	CategoryPC0day     = 4010
	CategoryPCISO      = 4020
	CategoryPCMac      = 4030
	CategoryPCMobileOther = 4040
	CategoryPCGames    = 4050
	CategoryPCMobileiOS= 4060
	CategoryPCMobileAndroid = 4070

	// TV
	CategoryTV         = 5000
	CategoryTVWEBDL    = 5010
	CategoryTVForeign  = 5020
	CategoryTVSD       = 5030
	CategoryTVHD       = 5040
	CategoryTVUHD      = 5045
	CategoryTVOther    = 5050
	CategoryTVSport    = 5060
	CategoryTVAnime    = 5070
	CategoryTVDocumentary = 5080

	// XXX
	CategoryXXX        = 6000
	CategoryXXXDVD     = 6010
	CategoryXXXWMV     = 6020
	CategoryXXXXviD    = 6030
	CategoryXXXx264    = 6040
	CategoryXXXUHD     = 6045
	CategoryXXXPack    = 6050
	CategoryXXXImageset= 6060
	CategoryXXXOther   = 6070

	// Books
	CategoryBooks      = 7000
	CategoryBooksMags  = 7010
	CategoryBooksEBook = 7020
	CategoryBooksComics= 7030
	CategoryBooksTechnical = 7040
	CategoryBooksOther = 7050
	CategoryBooksForeign = 7060

	// Other
	CategoryOther      = 8000
	CategoryOtherMisc  = 8010
	CategoryOtherHashed= 8020
)

// Category represents a Torznab category
type Category struct {
	ID          int
	Name        string
	Description string
	SubCats     []Category
}

// AllCategories returns all supported categories
func AllCategories() []Category {
	return []Category{
		{
			ID:   CategoryConsole,
			Name: "Console",
			SubCats: []Category{
				{ID: CategoryConsoleNDS, Name: "Console/NDS"},
				{ID: CategoryConsolePSP, Name: "Console/PSP"},
				{ID: CategoryConsoleWii, Name: "Console/Wii"},
				{ID: CategoryConsoleXbox, Name: "Console/Xbox"},
				{ID: CategoryConsoleXbox360, Name: "Console/Xbox360"},
				{ID: CategoryConsolePS3, Name: "Console/PS3"},
				{ID: CategoryConsolePS4, Name: "Console/PS4"},
				{ID: CategoryConsoleOther, Name: "Console/Other"},
			},
		},
		{
			ID:   CategoryMovies,
			Name: "Movies",
			SubCats: []Category{
				{ID: CategoryMoviesForeign, Name: "Movies/Foreign"},
				{ID: CategoryMoviesOther, Name: "Movies/Other"},
				{ID: CategoryMoviesSD, Name: "Movies/SD"},
				{ID: CategoryMoviesHD, Name: "Movies/HD"},
				{ID: CategoryMoviesUHD, Name: "Movies/UHD"},
				{ID: CategoryMovies3D, Name: "Movies/3D"},
				{ID: CategoryMoviesBluRay, Name: "Movies/BluRay"},
				{ID: CategoryMoviesDVD, Name: "Movies/DVD"},
				{ID: CategoryMoviesWEBDL, Name: "Movies/WEB-DL"},
			},
		},
		{
			ID:   CategoryAudio,
			Name: "Audio",
			SubCats: []Category{
				{ID: CategoryAudioMP3, Name: "Audio/MP3"},
				{ID: CategoryAudioVideo, Name: "Audio/Video"},
				{ID: CategoryAudioAudiobook, Name: "Audio/Audiobook"},
				{ID: CategoryAudioLossless, Name: "Audio/Lossless"},
				{ID: CategoryAudioOther, Name: "Audio/Other"},
			},
		},
		{
			ID:   CategoryPC,
			Name: "PC",
			SubCats: []Category{
				{ID: CategoryPC0day, Name: "PC/0day"},
				{ID: CategoryPCISO, Name: "PC/ISO"},
				{ID: CategoryPCMac, Name: "PC/Mac"},
				{ID: CategoryPCGames, Name: "PC/Games"},
				{ID: CategoryPCMobileiOS, Name: "PC/Mobile-iOS"},
				{ID: CategoryPCMobileAndroid, Name: "PC/Mobile-Android"},
			},
		},
		{
			ID:   CategoryTV,
			Name: "TV",
			SubCats: []Category{
				{ID: CategoryTVWEBDL, Name: "TV/WEB-DL"},
				{ID: CategoryTVForeign, Name: "TV/Foreign"},
				{ID: CategoryTVSD, Name: "TV/SD"},
				{ID: CategoryTVHD, Name: "TV/HD"},
				{ID: CategoryTVUHD, Name: "TV/UHD"},
				{ID: CategoryTVOther, Name: "TV/Other"},
				{ID: CategoryTVSport, Name: "TV/Sport"},
				{ID: CategoryTVAnime, Name: "TV/Anime"},
				{ID: CategoryTVDocumentary, Name: "TV/Documentary"},
			},
		},
		{
			ID:   CategoryXXX,
			Name: "XXX",
			SubCats: []Category{
				{ID: CategoryXXXDVD, Name: "XXX/DVD"},
				{ID: CategoryXXXx264, Name: "XXX/x264"},
				{ID: CategoryXXXUHD, Name: "XXX/UHD"},
				{ID: CategoryXXXOther, Name: "XXX/Other"},
			},
		},
		{
			ID:   CategoryBooks,
			Name: "Books",
			SubCats: []Category{
				{ID: CategoryBooksMags, Name: "Books/Mags"},
				{ID: CategoryBooksEBook, Name: "Books/EBook"},
				{ID: CategoryBooksComics, Name: "Books/Comics"},
				{ID: CategoryBooksTechnical, Name: "Books/Technical"},
				{ID: CategoryBooksOther, Name: "Books/Other"},
			},
		},
		{
			ID:   CategoryOther,
			Name: "Other",
			SubCats: []Category{
				{ID: CategoryOtherMisc, Name: "Other/Misc"},
			},
		},
	}
}

// CategoryName returns the name for a category ID
func CategoryName(id int) string {
	categories := map[int]string{
		1000: "Console",
		2000: "Movies",
		3000: "Audio",
		4000: "PC",
		5000: "TV",
		6000: "XXX",
		7000: "Books",
		8000: "Other",
	}

	// Check main categories
	base := (id / 1000) * 1000
	if name, ok := categories[base]; ok {
		return name
	}

	return "Other"
}

// MapNostrCategory maps a Nostr category tag to Torznab category ID
func MapNostrCategory(tag string) int {
	mapping := map[string]int{
		"movie":     CategoryMovies,
		"movies":    CategoryMovies,
		"film":      CategoryMovies,
		"tv":        CategoryTV,
		"series":    CategoryTV,
		"show":      CategoryTV,
		"anime":     CategoryTVAnime,
		"music":     CategoryAudio,
		"audio":     CategoryAudio,
		"games":     CategoryPCGames,
		"game":      CategoryPCGames,
		"software":  CategoryPC,
		"app":       CategoryPC,
		"apps":      CategoryPC,
		"books":     CategoryBooks,
		"book":      CategoryBooks,
		"ebook":     CategoryBooksEBook,
		"magazine":  CategoryBooksMags,
		"xxx":       CategoryXXX,
		"adult":     CategoryXXX,
		"porn":      CategoryXXX,
		"other":     CategoryOther,
	}

	if id, ok := mapping[tag]; ok {
		return id
	}

	return CategoryOther
}

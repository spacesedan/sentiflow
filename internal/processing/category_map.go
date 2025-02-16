package processing

var CategoryToSubreddits = map[string][]string{
	"Technology": {
		"technology",  // 13+ million
		"Futurology",  // 19+ million
		"programming", // 4.9+ million
		"gadgets",     // 16+ million
		"techsupport",
	},
	"Business & Finance": {
		"wallstreetbets",  // 13+ million
		"investing",       // 2.5+ million
		"finance",         // 1.6+ million
		"personalfinance", // 17+ million
		"entrpreneur",
	},
	"Politics & World Affairs": {
		"politics",    // 8+ million
		"worldnews",   // 31+ million
		"geopolitics", // 200k+
		"PoliticalHumor",
		"PoliticalDiscussion",
	},
	"Entertainment & Pop Culture": {
		"movies",         // 29+ million
		"television",     // 24+ million
		"popculturechat", // 500k+
		"music",          // 29+ million
	},
	"Health & Science": {
		"science",    // 30+ million
		"askscience", // 21+ million
		"health",     // 2+ million
		"nutrition",
		"medicine", // 500k+
	},
	"Sports": {
		"sports", // 28+ million
		"nba",    // 6.9+ million
		"nfl",    // 3.8+ million
		"soccer", // 4.6+ million
		"baseball",
	},
	"Lifestyle & Society": {
		"relationships",   // 3.3+ million
		"selfimprovement", // 1.6+ million
		"lifeprotips",     // 21+ million
		"socialskills",    // 3.4+ million
		"relationship_advice",
	},
	"Memes & Internet Trends": {
		"memes",        // 22+ million
		"dankmemes",    // 7+ million
		"me_irl",       // 4.4+ million
		"OutOfTheLoop", // 3.3+ million
		"PoliticalHumor",
	},
	"Crime & Law": {
		"legaladvice", // 2.6+ million
		"TrueCrime",   // 1.6+ million
		"law",         // 225k
		"CrimeScene",
	},
}

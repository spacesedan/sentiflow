package sentiment

import (
	"regexp"
	"strings"

	"github.com/jonreiter/govader"
	"github.com/russross/blackfriday/v2"
)

var analyzer = govader.NewSentimentIntensityAnalyzer()

func RemoveLinks(input string) string {
	linkPattern := regexp.MustCompile(`\[(.*?)\]\((https?:\/\/[^\s\)]+)\)`)
	input = linkPattern.ReplaceAllString(input, "$1") // Keep only the text

	urlPattern := regexp.MustCompile(`https?://\S+|www\.\S+`)
	input = urlPattern.ReplaceAllString(input, "")

	return input
}

func ConvertMarkdownToText(input string) string {
	output := blackfriday.Run([]byte(input), blackfriday.WithNoExtensions())
	plainText := strings.Join(strings.Fields(string(output)), " ")

	return RemoveLinks(plainText)
}

func AnalyzeWithVADER(text string) (float64, string) {
	plainText := ConvertMarkdownToText(text)

	sentiment := analyzer.PolarityScores(plainText)
	score := sentiment.Compound

	var label string
	if score >= 0.20 {
		label = "positive"
	} else if score <= -0.20 {
		label = "negative"
	} else {
		label = "neutral"
	}

	return score, label
}

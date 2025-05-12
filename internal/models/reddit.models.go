package models

import "time"

type RedditPost struct {
	Query       string    `json:"query"`
	Subreddit   string    `json:"subreddit"`
	Author      string    `json:"author"`
	PostTitle   string    `json:"post_title"`
	PostContent string    `json:"post_content"`
	Upvotes     int       `json:"upvotes"`
	CreatedAt   time.Time `json:"created_at"`
	PostID      string    `json:"id"`
}

type RedditAPIResponse struct {
	Data RedditAPIData `json:"data"`
}

type RedditAPIData struct {
	After    string           `json:"after"`
	Children []RedditAPIChild `json:"children"`
}

type RedditAPIChild struct {
	Data RedditAPIClildData `json:"data"`
}

type RedditAPIClildData struct {
	Subreddit      string  `json:"subreddit"`
	AuthorFullname string  `json:"author_fullname"`
	Title          string  `json:"title"`
	Selftext       string  `json:"selftext"`
	Ups            int     `json:"ups"`
	CreatedUTC     float64 `json:"created_utc"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
}

package models

import "encoding/xml"

type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        RSSGUID `xml:"guid"`
	Description string  `xml:"description"`
	PubDate     string  `xml:"pubDate"`
}

type RSSGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

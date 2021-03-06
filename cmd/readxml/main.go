package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	postDirectoryName = "posts"
	headerFormat      = `---
title: "%s"
date: %s
draft: false
---
`
	linkMatcher   = `https://confluence.int.aurora.tech(/x/[\w-_]+)`
	newLinkFormat = `../%s`
)

type Property struct {
	XMLName xml.Name `xml:"property"`
	Name    string   `xml:"name,attr"`
	Class   string   `xml:"class,attr"`
	Package string   `xml:"package,attr"`
	Id      string   `xml:"id"`
	Data    string   `xml:",chardata"`
}

type Object struct {
	XMLName    xml.Name   `xml:"object"`
	Class      string     `xml:"class,attr"`
	Package    string     `xml:"package,attr"`
	Properties []Property `xml:"property"`
	Id         string     `xml:"id"`
	Data       string     `xml:",chardata"`
}

type HibernateGeneric struct {
	XMLName  xml.Name `xml:"hibernate-generic"`
	DateTime string   `xml:"datetime,attr"`
	Id       string   `xml:"id"`
	Objects  []Object `xml:"object"`
	Data     string   `xml:",chardata"`
}

type BlogPost struct {
	Title  string
	Body   string
	BodyId string
	Date   time.Time
}

func main() {

	titleMap, linkMap := getPostInfo()

	// Open our xmlFile
	//	xmlFile, err := os.Open("c:\\Users\\leon\\repos\\confluence\\cmd\\readxml\\test.xml")
	xmlFile, err := os.Open("c:\\Users\\leon\\Downloads\\Confluence\\entities.xml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer xmlFile.Close()

	fmt.Println("Successfully Opened users.xml")

	data, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var hg HibernateGeneric

	err = xml.Unmarshal(data, &hg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	objectMapping := map[string]Object{}
	posts := map[string]BlogPost{}
	bodies := map[string]string{}

	for _, obj := range hg.Objects {
		objectMapping[obj.Id] = obj
		if obj.Class == "BlogPost" {
			parseBlogPost(obj, posts)
		}
		if obj.Class == "BodyContent" {
			parseBodyContent(obj, bodies)
		}
	}

	blogPosts := fixupPosts(posts, titleMap, linkMap, bodies)

	sort.Slice(blogPosts, func(i, j int) bool {
		return blogPosts[i].Date.Before(blogPosts[j].Date)
	})

	os.RemoveAll(postDirectoryName)
	os.Mkdir(postDirectoryName, 0777)
	for idx, post := range blogPosts {
		fmt.Printf("\t%d: `%s` created on %s\n", idx, post.Title, post.Date)
		baseName := "posts\\" + post.Date.Format("2006-01-02")
		fileName := baseName + ".html"
		_, err := os.Stat(fileName)
		ext := 0
		for err == nil {
			ext++
			fileName = fmt.Sprintf("%s_%d.html", baseName, ext)
			_, err = os.Stat(fileName)
		}
		file, err := os.Create(fileName)
		if err != nil {
			fmt.Println(err)
		}

		file.WriteString(fmt.Sprintf(headerFormat, post.Title, post.Date.Format("2006-01-02")))
		file.WriteString(post.Body)
		file.Close()
	}
}

func fixupPosts(posts map[string]BlogPost, titleMap map[string]time.Time, linkMap map[string]time.Time, bodies map[string]string) []BlogPost {
	fmt.Printf("Found %d blogs\n", len(posts))
	blogPosts := make([]BlogPost, 0, len(posts))
	for _, post := range posts {
		p := post
		info, hasDate := titleMap[p.Title]
		if hasDate {
			p.Date = info
		}
		body, found := bodies[p.BodyId]
		if !found {
			fmt.Printf("Couldn't find a body for post `%s`\n", p.Title)
		}

		p.Body = updateLinks(body, linkMap)
		blogPosts = append(blogPosts, p)
	}
	return blogPosts
}

func parseBodyContent(obj Object, bodies map[string]string) {
	var id string
	var body string

	for _, prop := range obj.Properties {
		data := prop.Data
		if prop.Name == "content" {
			id = prop.Id
		} else if prop.Name == "body" {
			body = data
		}
	}

	bodies[id] = body
}

func parseBlogPost(obj Object, posts map[string]BlogPost) {
	post := BlogPost{}
	var err error
	for _, prop := range obj.Properties {
		data := prop.Data
		if prop.Name == "title" {
			post.Title = strings.Replace(data, "\"", "'", -1)
		} else if prop.Name == "creationDate" {
			post.Date, err = time.Parse("2006-01-02 15:04:05", data)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	post.BodyId = obj.Id

	old, found := posts[post.Title]
	if post.Title != "" && (!found || old.Date.Before(post.Date)) {
		posts[post.Title] = post
	}
}

func getPostInfo() (map[string]time.Time, map[string]time.Time) {
	linkFile, err := os.Open("c:\\Users\\leon\\Downloads\\blogDates.txt")

	titleMap := map[string]time.Time{}
	linkMap := map[string]time.Time{}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer linkFile.Close()

	fileScanner := bufio.NewScanner(linkFile)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		parts := strings.Split(fileScanner.Text(), ":")
		title := parts[0]
		date, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		link := parts[3]

		titleMap[title] = date
		linkMap[link] = date
	}

	return titleMap, linkMap
}

func updateLinks(original string, linkMap map[string]time.Time) string {

	newBody := original
	re := regexp.MustCompile(linkMatcher)

	matches := re.FindAllStringSubmatch(original, -1)
	for idx, val := range matches {
		match := val[0]
		capture := val[1]
		info, found := linkMap[capture]
		if !found {
			fmt.Printf("Unable to find match for %d: match:`%s` group:'%s'\n", idx, match, capture)
			break
		}
		newLink := fmt.Sprintf(newLinkFormat, info.Format("2006-01-02"))
		newBody = strings.Replace(newBody, match, newLink, -1)
	}
	return newBody
}

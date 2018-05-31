package main

import (
	"io/ioutil"
	"testing"
)

func TestUser(t *testing.T) {

	b, _ := ioutil.ReadFile("tests/user.html")
	data, _ := GetUserFromMarkup(b)
	user := data.(*User)
	if user.Username != "petrus.malherbe" {
		t.Error("Incorrect username")
	}

	if len(user.Posts) != 12 {
		t.Error("Did not find 12 posts")
	}

}

func TestTag(t *testing.T) {
	b, _ := ioutil.ReadFile("tests/tag.html")
	data, _ := GetTagFromMarkup(b)
	tag := data.(*Tag)

	if tag.Name != "malherbe" {
		t.Error("Wrong tag name")
	}

	if len(tag.Posts) != 55 {
		t.Errorf("Incorrect number of posts for tag [%v]", len(tag.Posts))
	}
}

func TestLinkify(t *testing.T) {

	s := "@petrus.malherbe"

	markup := linkify(s)

	if string(markup) != "<a href=\"/user/petrus.malherbe\">@petrus.malherbe</a>" {
		t.Error("User with dot in username not parsed correctly", markup)
	}

	s = "#lovačkakuća"

	markup = linkify(s)

	if string(markup) != "<a href=\"/tag/lovačkakuća\">#lovačkakuća</a>" {
		t.Error("Tag with non-ASCII not parsed correctly", markup)
	}

}

func TestMaxSize(t *testing.T) {

	sizes := []Size{
		{Width:200, Height:200},
		{Width:400, Height: 400},
		{Width:600, Height:600},
	}

	p := &Post{Sizes:sizes}

	s := sizemax(p, 500)

	if s.Width != 400 {
		t.Error("Expected size with width 400, not", s.Width)
	}

}

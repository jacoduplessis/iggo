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

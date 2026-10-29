package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }

func TestCurrentPublishInput(t *testing.T) {
	toc, delisted := true, false
	input := PublishPostInput{
		Title:           "A better way to live",
		PublicationId:   "publication-id",
		ContentMarkdown: "Article body",
		Slug:            stringPtr("better-way"),
		Tags: []PublishPostTagInput{
			{Name: stringPtr("Philosophy"), Slug: stringPtr("philosophy")},
		},
		MetaTags: &MetaTagsInput{Title: stringPtr("SEO title"), Description: stringPtr("SEO description")},
		Settings: &PublishPostSettingsInput{
			EnableTableOfContent: &toc,
			Delisted:             &delisted,
			SlugOverridden:       &toc,
		},
		CoverImageOptions: &CoverImageOptionsInput{CoverImageURL: stringPtr("https://example.com/image.jpg")},
	}
	data, err := currentPublishInput(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["enableToc"] != true || decoded["isDelisted"] != false ||
		decoded["metaTitle"] != "SEO title" || decoded["metaDescription"] != "SEO description" ||
		decoded["coverImage"] != "https://example.com/image.jpg" {
		t.Fatalf("incorrect current API variables: %s", raw)
	}
	for _, obsolete := range []string{"settings", "metaTags", "coverImageOptions", "bannerImageOptions"} {
		if _, exists := decoded[obsolete]; exists {
			t.Fatalf("obsolete field %s sent: %s", obsolete, raw)
		}
	}
	tags, ok := decoded["tags"].([]any)
	if !ok || len(tags) != 1 {
		t.Fatalf("incorrect tags: %s", raw)
	}
	tag := tags[0].(map[string]any)
	if tag["slug"] != "philosophy" || tag["name"] != "Philosophy" {
		t.Fatalf("incorrect tag: %#v", tag)
	}
	if _, exists := tag["id"]; exists {
		t.Fatalf("obsolete ID field in tag: %#v", tag)
	}
}

func TestCurrentUpdateInput(t *testing.T) {
	toc, comments, delisted := true, false, true
	data, err := currentUpdateInput(UpdatePostInput{
		Id:              "post-id",
		ContentMarkdown: stringPtr("Updated body"),
		Settings: &UpdatePostSettingsInput{
			IsTableOfContentEnabled: &toc,
			DisableComments:         &comments,
			Delisted:                &delisted,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if data["id"] != "post-id" || data["contentMarkdown"] != "Updated body" ||
		data["enableToc"] != true || data["disableComments"] != false || data["isDelisted"] != true {
		t.Fatalf("incorrect update variables: %#v", data)
	}
}

func TestUnsupportedFieldsFailClearly(t *testing.T) {
	_, err := currentPublishInput(PublishPostInput{
		Settings: &PublishPostSettingsInput{IsNewsletterActivated: boolPtr(true)},
	})
	if err == nil || !strings.Contains(err.Error(), "newsletter") {
		t.Fatalf("expected unsupported newsletter error; got %v", err)
	}
	_, err = currentPublishInput(PublishPostInput{BannerImageOptions: &BannerImageOptionsInput{}})
	if err == nil || !strings.Contains(err.Error(), "banner") {
		t.Fatalf("expected unsupported banner error; got %v", err)
	}
	_, err = currentUpdateInput(UpdatePostInput{
		Settings: &UpdatePostSettingsInput{PinToBlog: boolPtr(true)},
	})
	if err == nil || !strings.Contains(err.Error(), "pin_to_blog") {
		t.Fatalf("expected unsupported pin error; got %v", err)
	}
}

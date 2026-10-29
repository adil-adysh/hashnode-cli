package api

import (
	"context"
	"fmt"
	"time"

	"github.com/Khan/genqlient/graphql"
)

// The historical generated input types are kept for the existing frontmatter
// mapping, but Hashnode's current publish/update mutations use flat metadata,
// SEO and settings fields. Build their variables explicitly to avoid sending
// removed fields (even as JSON nulls) to the API.
func PublishPostCurrent(ctx context.Context, client graphql.Client, input PublishPostInput) (*PublishPostResponse, error) {
	vars, err := currentPublishInput(input)
	if err != nil {
		return nil, err
	}
	response := &PublishPostResponse{}
	err = client.MakeRequest(ctx, &graphql.Request{
		OpName: "PublishPost",
		Query:  PublishPost_Operation,
		Variables: map[string]any{"input": vars},
	}, &graphql.Response{Data: response})
	return response, err
}

func UpdatePostCurrent(ctx context.Context, client graphql.Client, input UpdatePostInput) (*UpdatePostResponse, error) {
	vars, err := currentUpdateInput(input)
	if err != nil {
		return nil, err
	}
	response := &UpdatePostResponse{}
	err = client.MakeRequest(ctx, &graphql.Request{
		OpName: "UpdatePost",
		Query:  UpdatePost_Operation,
		Variables: map[string]any{"input": vars},
	}, &graphql.Response{Data: response})
	return response, err
}

func currentPublishInput(in PublishPostInput) (map[string]any, error) {
	if in.DraftId != nil {
		return nil, fmt.Errorf("publishing an existing draft requires publishDraft, not publishPost")
	}
	out := map[string]any{
		"publicationId":  in.PublicationId,
		"title":          in.Title,
		"contentMarkdown": in.ContentMarkdown,
	}
	setString(out, "subtitle", in.Subtitle)
	setString(out, "slug", in.Slug)
	setString(out, "originalArticleURL", in.OriginalArticleURL)
	setString(out, "publishAs", in.PublishAs)
	setString(out, "seriesId", in.SeriesId)
	setBool(out, "disableComments", in.DisableComments)
	setTime(out, "publishedAt", in.PublishedAt)
	if len(in.CoAuthors) > 0 {
		out["coAuthors"] = in.CoAuthors
	}
	if err := addPostMedia(out, in.CoverImageOptions, in.BannerImageOptions); err != nil {
		return nil, err
	}
	addPostMeta(out, in.MetaTags)
	tags, err := currentTags(in.Tags)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		out["tags"] = tags
	}
	if in.Settings != nil {
		setBool(out, "enableToc", in.Settings.EnableTableOfContent)
		setBool(out, "isDelisted", in.Settings.Delisted)
		if in.Settings.Scheduled != nil || in.Settings.IsNewsletterActivated != nil {
			return nil, fmt.Errorf("scheduled/newsletter publishing settings are not supported by the current publishPost API")
		}
		// SlugOverridden is now inferred from an explicit slug.
	}
	return out, nil
}

func currentUpdateInput(in UpdatePostInput) (map[string]any, error) {
	out := map[string]any{"id": in.Id}
	setString(out, "publicationId", in.PublicationId)
	setString(out, "title", in.Title)
	setString(out, "subtitle", in.Subtitle)
	setString(out, "contentMarkdown", in.ContentMarkdown)
	setString(out, "slug", in.Slug)
	setString(out, "originalArticleURL", in.OriginalArticleURL)
	setString(out, "publishAs", in.PublishAs)
	setString(out, "seriesId", in.SeriesId)
	setTime(out, "publishedAt", in.PublishedAt)
	if len(in.CoAuthors) > 0 {
		out["coAuthors"] = in.CoAuthors
	}
	if err := addPostMedia(out, in.CoverImageOptions, in.BannerImageOptions); err != nil {
		return nil, err
	}
	addPostMeta(out, in.MetaTags)
	tags, err := currentTags(in.Tags)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		out["tags"] = tags
	}
	if in.Settings != nil {
		setBool(out, "enableToc", in.Settings.IsTableOfContentEnabled)
		setBool(out, "isDelisted", in.Settings.Delisted)
		setBool(out, "disableComments", in.Settings.DisableComments)
		if in.Settings.PinToBlog != nil {
			return nil, fmt.Errorf("pin_to_blog is not supported by the current updatePost API")
		}
	}
	return out, nil
}

func currentTags(tags []PublishPostTagInput) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		if tag.Id != nil || tag.Slug == nil || *tag.Slug == "" {
			return nil, fmt.Errorf("Hashnode tags must have a slug; ID-only tags are no longer supported")
		}
		t := map[string]any{"slug": *tag.Slug}
		setString(t, "name", tag.Name)
		result = append(result, t)
	}
	return result, nil
}

func addPostMedia(out map[string]any, cover *CoverImageOptionsInput, banner *BannerImageOptionsInput) error {
	if banner != nil {
		return fmt.Errorf("banner_image_url is not supported by the current post API")
	}
	if cover == nil {
		return nil
	}
	if cover.CoverImageURL == nil {
		return fmt.Errorf("cover image metadata requires cover_image_url")
	}
	if cover.CoverImageAttribution != nil || cover.CoverImagePhotographer != nil ||
		cover.IsCoverAttributionHidden != nil || cover.StickCoverToBottom != nil {
		return fmt.Errorf("cover image attribution/position settings are not supported by the current post API")
	}
	out["coverImage"] = *cover.CoverImageURL
	return nil
}

func addPostMeta(out map[string]any, meta *MetaTagsInput) {
	if meta == nil {
		return
	}
	setString(out, "metaTitle", meta.Title)
	setString(out, "metaDescription", meta.Description)
	setString(out, "ogImage", meta.Image)
}

func setString(out map[string]any, key string, value *string) {
	if value != nil {
		out[key] = *value
	}
}

func setBool(out map[string]any, key string, value *bool) {
	if value != nil {
		out[key] = *value
	}
}

func setTime(out map[string]any, key string, value *time.Time) {
	if value != nil {
		out[key] = value
	}
}

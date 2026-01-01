package applyutil

import (
	"strings"

	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/state"
)

// Apply frontmatter metadata to a publish input. Nil frontmatter is a no-op.
func ApplyFrontmatterToPublishInput(input *api.PublishPostInput, fm *state.Frontmatter, sum *state.Sum) {
	if fm == nil {
		return
	}

	if fm.Subtitle != "" {
		input.Subtitle = strPtr(fm.Subtitle)
	}
	if fm.Slug != "" {
		input.Slug = strPtr(fm.Slug)
	}
	if fm.Canonical != "" {
		input.OriginalArticleURL = strPtr(fm.Canonical)
	}
	if fm.PublishedAt != nil {
		input.PublishedAt = fm.PublishedAt
	}
	if fm.DisableComments != nil {
		input.DisableComments = fm.DisableComments
	}

	if fm.CoverImageURL != "" || fm.CoverImageAttribution != "" || fm.CoverImagePhotographer != "" || fm.CoverImageHideAttribution || fm.CoverImageStickBottom {
		input.CoverImageOptions = &api.CoverImageOptionsInput{
			CoverImageURL:            strPtrOrNil(fm.CoverImageURL),
			CoverImageAttribution:    strPtrOrNil(fm.CoverImageAttribution),
			CoverImagePhotographer:   strPtrOrNil(fm.CoverImagePhotographer),
			IsCoverAttributionHidden: boolPtrOrNil(fm.CoverImageHideAttribution),
			StickCoverToBottom:       boolPtrOrNil(fm.CoverImageStickBottom),
		}
	}
	if fm.BannerImageURL != "" {
		input.BannerImageOptions = &api.BannerImageOptionsInput{BannerImageURL: strPtr(fm.BannerImageURL)}
	}

	if fm.MetaTitle != "" || fm.MetaDescription != "" || fm.MetaImage != "" {
		input.MetaTags = &api.MetaTagsInput{
			Title:       strPtrOrNil(fm.MetaTitle),
			Description: strPtrOrNil(fm.MetaDescription),
			Image:       strPtrOrNil(fm.MetaImage),
		}
	}

	if len(fm.Tags) > 0 {
		input.Tags = append(input.Tags, tagsToInputs(fm.Tags)...)
	}

	if fm.PublishAs != "" {
		input.PublishAs = strPtr(fm.PublishAs)
	}
	if len(fm.CoAuthors) > 0 {
		input.CoAuthors = append(input.CoAuthors, fm.CoAuthors...)
	}

	if fm.EnableToc != nil || fm.Newsletter != nil || fm.Delisted != nil || fm.Scheduled != nil || fm.SlugOverridden != nil || fm.Slug != "" {
		settings := api.PublishPostSettingsInput{}
		if fm.EnableToc != nil {
			settings.EnableTableOfContent = fm.EnableToc
		}
		if fm.Newsletter != nil {
			settings.IsNewsletterActivated = fm.Newsletter
		}
		if fm.Delisted != nil {
			settings.Delisted = fm.Delisted
		}
		if fm.Scheduled != nil {
			settings.Scheduled = fm.Scheduled
		}
		if fm.SlugOverridden != nil {
			settings.SlugOverridden = fm.SlugOverridden
		} else if fm.Slug != "" {
			settings.SlugOverridden = boolPtr(true)
		}
		input.Settings = &settings
	}

	if fm.Series != "" {
		if sid := resolveSeriesID(fm.Series, sum); sid != "" {
			input.SeriesId = &sid
		}
	}
}

// Apply frontmatter metadata to an update input. Nil frontmatter is a no-op.
func ApplyFrontmatterToUpdateInput(input *api.UpdatePostInput, fm *state.Frontmatter, sum *state.Sum) {
	if fm == nil {
		return
	}

	if fm.Subtitle != "" {
		input.Subtitle = strPtr(fm.Subtitle)
	}
	if fm.Slug != "" {
		input.Slug = strPtr(fm.Slug)
	}
	if fm.Canonical != "" {
		input.OriginalArticleURL = strPtr(fm.Canonical)
	}
	if fm.PublishedAt != nil {
		input.PublishedAt = fm.PublishedAt
	}

	if fm.CoverImageURL != "" || fm.CoverImageAttribution != "" || fm.CoverImagePhotographer != "" || fm.CoverImageHideAttribution || fm.CoverImageStickBottom {
		input.CoverImageOptions = &api.CoverImageOptionsInput{
			CoverImageURL:            strPtrOrNil(fm.CoverImageURL),
			CoverImageAttribution:    strPtrOrNil(fm.CoverImageAttribution),
			CoverImagePhotographer:   strPtrOrNil(fm.CoverImagePhotographer),
			IsCoverAttributionHidden: boolPtrOrNil(fm.CoverImageHideAttribution),
			StickCoverToBottom:       boolPtrOrNil(fm.CoverImageStickBottom),
		}
	}
	if fm.BannerImageURL != "" {
		input.BannerImageOptions = &api.BannerImageOptionsInput{BannerImageURL: strPtr(fm.BannerImageURL)}
	}

	if fm.MetaTitle != "" || fm.MetaDescription != "" || fm.MetaImage != "" {
		input.MetaTags = &api.MetaTagsInput{
			Title:       strPtrOrNil(fm.MetaTitle),
			Description: strPtrOrNil(fm.MetaDescription),
			Image:       strPtrOrNil(fm.MetaImage),
		}
	}

	if len(fm.Tags) > 0 {
		input.Tags = append(input.Tags, tagsToInputs(fm.Tags)...)
	}

	if fm.PublishAs != "" {
		input.PublishAs = strPtr(fm.PublishAs)
	}
	if len(fm.CoAuthors) > 0 {
		input.CoAuthors = append(input.CoAuthors, fm.CoAuthors...)
	}

	if fm.EnableToc != nil || fm.Delisted != nil || fm.DisableComments != nil || fm.PinToBlog != nil {
		settings := api.UpdatePostSettingsInput{}
		if fm.EnableToc != nil {
			settings.IsTableOfContentEnabled = fm.EnableToc
		}
		if fm.Delisted != nil {
			settings.Delisted = fm.Delisted
		}
		if fm.DisableComments != nil {
			settings.DisableComments = fm.DisableComments
		}
		if fm.PinToBlog != nil {
			settings.PinToBlog = fm.PinToBlog
		}
		input.Settings = &settings
	}

	if fm.Series != "" {
		if sid := resolveSeriesID(fm.Series, sum); sid != "" {
			input.SeriesId = &sid
		}
	}
}

func tagsToInputs(tags []string) []api.PublishPostTagInput {
	var out []api.PublishPostTagInput
	for _, t := range tags {
		name := strings.TrimSpace(t)
		if name == "" {
			continue
		}
		slug := slugifyTag(name)
		out = append(out, api.PublishPostTagInput{Name: &name, Slug: &slug})
	}
	return out
}

func slugifyTag(s string) string {
	s = strings.ToLower(s)
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "tag"
	}
	return clean
}

func resolveSeriesID(name string, sum *state.Sum) string {
	if sum == nil || len(sum.Series) == 0 {
		return ""
	}
	slug := state.SeriesSlug(name)
	for _, se := range sum.Series {
		if strings.EqualFold(se.Name, name) || strings.EqualFold(se.Slug, slug) {
			return se.SeriesID
		}
	}
	return ""
}

func strPtr(v string) *string { return &v }
func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool { return &v }
func boolPtrOrNil(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}

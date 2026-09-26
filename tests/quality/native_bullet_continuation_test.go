package quality

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
)

// Exercise the same production primitive used by repair_slide, retaining the
// native SlideSpec composition. This is not approval of the original dense page.
func nativeBulletContinuations(source nativeProbe, budget int) ([]nativeProbe, error) {
	base := deckinput.SlideInput{}
	markers := map[string]bool{}
	for _, item := range source.Slide.Content {
		ci := deckinput.ContentInput{Type: string(item.Type), PlaceholderID: item.PlaceholderID}
		if item.Type == generator.ContentBullets {
			bullets, ok := item.Value.([]string)
			if !ok {
				return nil, fmt.Errorf("native fixture needs typed bullet strings")
			}
			ci.BulletsValue = &bullets
			for _, bullet := range bullets {
				fields := strings.Fields(bullet)
				if len(fields) == 0 || markers[fields[0]] {
					return nil, fmt.Errorf("native fixture needs unique nonempty bullet markers")
				}
				markers[fields[0]] = true
			}
		}
		base.Content = append(base.Content, ci)
	}
	if len(markers) == 0 {
		return []nativeProbe{source}, nil
	}
	expanded, err := deckinput.SplitBulletSlide(base, budget)
	if err != nil {
		return nil, err
	}
	pages := make([]nativeProbe, len(expanded))
	for i, expandedPage := range expanded {
		page := source
		page.ID = fmt.Sprintf("%s-bullet-continuation-%02d", source.LayoutID, i+1)
		page.Profile = "bullet-continuation"
		page.Slide.Content = append([]generator.ContentItem(nil), source.Slide.Content...)
		page.ExpectedText = nil
		for _, text := range source.ExpectedText {
			if !markers[text] {
				page.ExpectedText = append(page.ExpectedText, text)
			}
		}
		for j, item := range expandedPage.Content {
			if item.Type != "bullets" {
				continue
			}
			page.Slide.Content[j].Value = append([]string(nil), (*item.BulletsValue)...)
			for _, bullet := range *item.BulletsValue {
				page.ExpectedText = append(page.ExpectedText, strings.Fields(bullet)[0])
				page.ExpectedText = append(page.ExpectedText, strings.TrimSpace(bullet))
			}
		}
		pages[i] = page
	}
	return pages, nil
}

func TestNativeBulletContinuationsPreserveAllLayoutsAndSource(t *testing.T) {
	count := 0
	for _, path := range nativeCorpusPaths(t) {
		r, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		layouts, err := template.ParseLayouts(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, layout := range layouts {
			for _, source := range makeNativeProbes(layout, nativeReferenceImage()) {
				if source.Profile != "dense" {
					continue
				}
				before, _ := json.Marshal(source)
				pages, err := nativeBulletContinuations(source, 3)
				if err != nil {
					t.Fatalf("%s/%s: %v", path, layout.ID, err)
				}
				if len(pages) > 1 {
					count++
				}
				for index, item := range source.Slide.Content {
					if item.Type != generator.ContentBullets {
						for _, page := range pages {
							if !reflect.DeepEqual(page.Slide.Content[index], item) {
								t.Fatal("non-bullet item changed")
							}
						}
						continue
					}
					var joined []string
					for _, page := range pages {
						want := source.Slide
						want.Content = page.Slide.Content
						if !reflect.DeepEqual(want, page.Slide) || page.ExpectedPictures != source.ExpectedPictures || page.ExpectedTables != source.ExpectedTables {
							t.Fatal("native layout/composition/inventory changed")
						}
						joined = append(joined, page.Slide.Content[index].Value.([]string)...)
						for _, bullet := range page.Slide.Content[index].Value.([]string) {
							marker := strings.Fields(bullet)[0]
							found := false
							for _, expected := range page.ExpectedText {
								found = found || marker == expected
							}
							if !found {
								t.Fatal("page source marker not checked")
							}
							fullSentence := false
							for _, expected := range page.ExpectedText {
								fullSentence = fullSentence || expected == strings.TrimSpace(bullet)
							}
							if !fullSentence {
								t.Fatal("full bullet sentence not independently checked")
							}
						}
					}
					if !reflect.DeepEqual(joined, item.Value) {
						t.Fatal("bullet strings omitted/rewritten/reordered")
					}
				}
				after, _ := json.Marshal(source)
				if string(before) != string(after) {
					t.Fatal("source fixture mutated")
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("no native bullet layouts exercised")
	}
	t.Logf("preserved dense bullet source on %d native layouts", count)
}

func TestNativeBulletContinuationsRejectPartialSourceSentences(t *testing.T) {
	const sentence = "C1-B00 Café teams retain 12% and do not omit the qualifier"
	source := nativeProbe{LayoutID: "native", Profile: "dense", ExpectedText: []string{"C1-B00"}, Slide: generator.SlideSpec{Content: []generator.ContentItem{{Type: generator.ContentBullets, Value: []string{sentence}}}}}
	pages, err := nativeBulletContinuations(source, 3)
	if err != nil {
		t.Fatal(err)
	}
	partial := []byte(`<p:sld xmlns:p="p" xmlns:a="a"><a:t>C1-B00 Café teams retain 12%</a:t></p:sld>`)
	failures := checkNativeProbeSlide(partial, pages[0])
	if len(failures) != 1 || !strings.Contains(failures[0], sentence) {
		t.Fatalf("marker-only partial sentence accepted: %v", failures)
	}
	complete := []byte(`<p:sld xmlns:p="p" xmlns:a="a"><a:t>` + sentence + `</a:t></p:sld>`)
	if failures := checkNativeProbeSlide(complete, pages[0]); len(failures) != 0 {
		t.Fatalf("complete sentence rejected: %v", failures)
	}
}

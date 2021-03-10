package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	ldk "github.com/open-olive/loop-development-kit/ldk/go"
)

// Serve creates the new loop and tells the LDK to serve it
func Serve() error {
	logger := ldk.NewLogger("venddy-search-searchbar")
	loop, err := NewLoop(logger)
	if err != nil {
		return err
	}
	ldk.ServeLoopPlugin(logger, loop)
	return nil
}

// Loop is a structure for generating SideKick whispers
type Loop struct {
	ctx    context.Context
	cancel context.CancelFunc

	sidekick ldk.Sidekick
	logger   *ldk.Logger
}

// NewLoop returns a pointer to a loop
func NewLoop(logger *ldk.Logger) (*Loop, error) {
	return &Loop{
		logger: logger,
	}, nil
}

var client = &http.Client{Timeout: 10 * time.Second}
var searchLimit = 10
var cursor = 0

// LoopStart is called by the host when the loop is started to provide access to the host process
func (l *Loop) LoopStart(sidekick ldk.Sidekick) error {
	l.logger.Info("starting venddy search")
	l.ctx, l.cancel = context.WithCancel(context.Background())

	l.sidekick = sidekick

	return sidekick.UI().ListenSearchbar(l.ctx, func(text string, err error) {
		l.logger.Info("loop callback called")
		if err != nil {
			l.logger.Error("received error from callback", err)
			return
		}

		go func() {
			resp, err := l.GetVendorSearch(text, searchLimit, cursor)
			if err != nil {
				log.Fatalln(err)
			}
			defer resp.Body.Close()

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			venddy := Venddy{}

			er := json.Unmarshal(bodyBytes, &venddy)
			if er != nil {
				log.Fatalln(er.Error())
			}

			venddy.Response.Results = l.GetVenddyCategoryNames(venddy.Response.Results)
			venddy.Response.Results = l.GetVenddyClassNames(venddy.Response.Results)

			go func() {
				_, err := l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
					Label:    "Venddy Search",
					Elements: l.CreateDisambiguationElements(venddy.Response, text),
				})
				if err != nil {
					l.logger.Error("failed to emit whisper", "error", err)
				}
			}()
		}()
	})
}

func (l *Loop) CreateDisambiguationElements(response VenddyResponse, text string) map[string]ldk.WhisperContentDisambiguationElement {
	elements := make(map[string]ldk.WhisperContentDisambiguationElement)

	for i := range response.Results {
		item := response.Results[i]
		elements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentDisambiguationElementOption{
			Label: fmt.Sprintf("%v ~ Rating:%.0f ~ Reviews:%.0f", item.Name, item.Score, item.ReviewCount),
			Order: uint32(i) + 1,
			OnChange: func(key string) {
				go func() {
					err := l.sidekick.Whisper().Markdown(l.ctx, &ldk.WhisperContentMarkdown{
						Label: item.Name,
						Markdown: fmt.Sprintf(`[![Logo not found](%v)](%v) `, item.Logo, item.Website) +
							"\n>" + item.Description + "\n" +
							"\n# Categories:\n" + item.CategoryNames +
							"\n# Classes:\n" + item.ClassNames,
					})

					if err != nil {
						log.Fatalln(err)
					}
				}()
			},
		}
	}

	elements["header1"] = &ldk.WhisperContentDisambiguationElementText{
		Body:  fmt.Sprintf("# Results for %v:", text),
		Order: 0,
	}
	elements["header2"] = &ldk.WhisperContentDisambiguationElementText{
		Body:  fmt.Sprintf("Remaining Results: %v", response.Remaining),
		Order: uint32(len(response.Results)) + 2,
	}

	if response.Remaining > 0 {
		elements["next"] = &ldk.WhisperContentDisambiguationElementOption{
			Label: fmt.Sprintf("next %v results", searchLimit),
			Order: uint32(len(response.Results)) + 3,
			OnChange: func(key string) {
				go func() {
					cursor += searchLimit
					resp, err := l.GetVendorSearch(text, searchLimit, cursor)
					if err != nil {
						log.Fatalln(err)
					}
					defer resp.Body.Close()

					bodyBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Fatalln(err)
					}

					venddy := Venddy{}

					er := json.Unmarshal(bodyBytes, &venddy)
					if er != nil {
						log.Fatalln(er.Error())
					}
					_, _ = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
						Label:    "Venddy Search",
						Elements: l.CreateDisambiguationElements(venddy.Response, text),
					})
				}()
			},
		}
	}

	if cursor > 0 {
		elements["prev"] = &ldk.WhisperContentDisambiguationElementOption{
			Label: fmt.Sprintf("prev %v results", searchLimit),
			Order: uint32(len(response.Results)) + 4,
			OnChange: func(key string) {
				go func() {
					cursor -= searchLimit
					resp, err := l.GetVendorSearch(text, searchLimit, cursor)
					if err != nil {
						l.logger.Error("GetVendorSearch failed", err)
					}
					defer resp.Body.Close()

					bodyBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						l.logger.Error("ioutil.ReadAll failed", err)
					}

					venddy := Venddy{}

					er := json.Unmarshal(bodyBytes, &venddy)
					if er != nil {
						l.logger.Error("json.Unmarshal failed", err)
					}
					_, _ = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
						Label:    "Venddy Search",
						Elements: l.CreateDisambiguationElements(venddy.Response, text),
					})
				}()
			},
		}
	}

	return elements
}

func CreateListElements(result VenddyResult) map[string]ldk.WhisperContentListElement {
	elements := map[string]ldk.WhisperContentListElement{}
	elements[fmt.Sprintf("%v_website", result.Id)] = &ldk.WhisperContentListElementLink{
		Align: ldk.WhisperContentListElementAlignLeft,
		Href:  result.Website,
		Order: 0,
		Style: ldk.WhisperContentListElementStyleNone,
		Text:  result.Website,
	}

	elements[fmt.Sprintf("%v_rating", result.Id)] = &ldk.WhisperContentListElementPair{
		Label: "Rating",
		Order: 1,
		Value: fmt.Sprintf("%v", result.Score),
	}

	elements[fmt.Sprintf("%v_reviews", result.Id)] = &ldk.WhisperContentListElementPair{
		Label: "Reivews",
		Order: 2,
		Value: fmt.Sprintf("%v", result.ReviewCount),
	}

	elements[fmt.Sprintf("%v_description", result.Id)] = &ldk.WhisperContentListElementMessage{
		Style:  ldk.WhisperContentListElementStyleNone,
		Header: result.Description,
		Body:   result.Keywords,
		Align:  ldk.WhisperContentListElementAlignLeft,
		Order:  3,
	}

	return elements
}

func (l *Loop) GetVendorSearch(vendor string, max int, startAt int) (*http.Response, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf(`https://venddy.com/api/1.1/obj/vendor?constraints=[{"key":"searchfield", "constraint_type":"text contains", "value":"%v" }]&sort_field=Score&descending=true&limit=%v&cursor=%v`,
			vendor, max, startAt), nil)
	if err != nil {
		l.logger.Error("Vendor GET failed", err)
	}
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	return client.Do(req)
}

func (l *Loop) GetVenddyCategoryNames(results []VenddyResult) []VenddyResult {
	venddyCategories := Venddy{}
	resp, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    "https://venddy.com/api/1.1/obj/category",
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Category GET failed", err)
	}

	err = json.Unmarshal(resp.Data, &venddyCategories)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}

	for i := range venddyCategories.Response.Results {
		for in := range results {
			for ind := range results[in].Categories {
				if venddyCategories.Response.Results[i].Id == results[in].Categories[ind] {
					results[in].CategoryNames += "- " + venddyCategories.Response.Results[i].Name + "\n"
				}
			}
		}
	}
	return results
}

func (l *Loop) GetVenddyClassNames(results []VenddyResult) []VenddyResult {
	venddyClasses := Venddy{}
	resp, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    "https://venddy.com/api/1.1/obj/class",
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Class GET failed", err)
	}

	err = json.Unmarshal(resp.Data, &venddyClasses)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}

	for i := range venddyClasses.Response.Results {
		for in := range results {
			for ind := range results[in].Classes {
				if venddyClasses.Response.Results[i].Id == results[in].Classes[ind] {
					results[in].ClassNames += "- " + venddyClasses.Response.Results[i].Name + "\n"
				}
			}
		}
	}
	return results
}

type Venddy struct {
	Response VenddyResponse `json:"response"`
}

type VenddyResponse struct {
	Results   []VenddyResult `json:"results"`
	Cursor    int            `json:"Cursor"`
	Remaining int            `json:"Remaining"`
	Count     int            `json:"Count"`
}

type VenddyResult struct {
	Id            string   `json:"_id"`
	Website       string   `json:"Website"`
	Description   string   `json:"Description"`
	Name          string   `json:"Name"`
	Logo          string   `json:"Logo"`
	Keywords      string   `json:"Search field"`
	Categories    []string `json:"Categories"`
	CategoryNames string
	Classes       []string `json:"Classes"`
	ClassNames    string
	Subcategories []string `json:"Subcategories"`
	Score         float64  `json:"Score"`
	ReviewCount   float64  `json:"Number of Reviews"`
}

// LoopStop is called by the host when the loop is stopped
func (l *Loop) LoopStop() error {
	l.logger.Info("LoopStop called")
	l.cancel()

	return nil
}

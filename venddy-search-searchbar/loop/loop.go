package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
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
			cursor = 0
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
			venddy.Response.Results = l.GetVenddySubcategoryNames(venddy.Response.Results)
			venddy.Response.Results = l.GetVenddyTypeNames(venddy.Response.Results)

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

	if len(response.Results) > 0 {
		for i := range response.Results {
			item := response.Results[i]
			elements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentDisambiguationElementOption{
				Label: fmt.Sprintf("%v ~ Rating:%.0f ~ Reviews:%.0f", item.Name, item.Score, item.ReviewCount),
				Order: uint32(i) + 1,
				OnChange: func(key string) {
					go func() {
						logo := item.Logo
						if logo == logo[:0] {
							logo = "https://d1muf25xaso8hp.cloudfront.net/https%3A%2F%2Fs3.amazonaws.com%2Fappforest_uf%2Ff1531944633470x300479865865781900%2FDefault%2520Logo.png?w=256&h=256&auto=compress&dpr=1&fit=max"
						}

						if logo[:1] != "h" {
							logo = fmt.Sprintf("https://d1muf25xaso8hp.cloudfront.net/http:%v", logo)
						}

						err := l.sidekick.Whisper().Markdown(l.ctx, &ldk.WhisperContentMarkdown{
							Label: item.Name,
							Markdown: fmt.Sprintf(`[![Logo not found](%v)](%v) `, logo, item.Website) +
								"\n_" + item.Description + "_\n" +
								"\n# Classes:\n" + item.ClassNames +
								"\n# Types:\n" + item.TypeNames +
								"\n# Categories:\n" + item.CategoryNames +
								"\n# Subcategories:\n" + item.SubcategoryNames +
								"\n\n " + fmt.Sprintf("[View on Venddy](https://venddy.com/vendorprofile/%v)", item.Id),
						})

						if err != nil {
							l.logger.Error("Whisper().Markdown() failed", err)
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
			Body:  fmt.Sprintf("# Remaining Results: %v", response.Remaining),
			Order: uint32(len(response.Results)) + 2,
		}
		venddyText := strings.ReplaceAll(text, " ", "+")
		elements["viewOnVenddy"] = &ldk.WhisperContentDisambiguationElementText{
			Body:  fmt.Sprintf("https://venddy.com/searchvendor?keyword=%v", venddyText),
			Order: uint32(len(response.Results)) + 5,
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
							l.logger.Error("GetVendorSearch failed", err)
						}
						defer resp.Body.Close()

						bodyBytes, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							l.logger.Error("ioutil.ReadAll failed", err)
						}

						venddy := Venddy{}

						err = json.Unmarshal(bodyBytes, &venddy)
						if err != nil {
							l.logger.Error("json.Unmarshal failed", err)
						}

						venddy.Response.Results = l.GetVenddyCategoryNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddyClassNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddySubcategoryNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddyTypeNames(venddy.Response.Results)

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

						venddy.Response.Results = l.GetVenddyCategoryNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddyClassNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddySubcategoryNames(venddy.Response.Results)
						venddy.Response.Results = l.GetVenddyTypeNames(venddy.Response.Results)

						_, _ = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
							Label:    "Venddy Search",
							Elements: l.CreateDisambiguationElements(venddy.Response, text),
						})
					}()
				},
			}
		}
	} else {
		elements["header1"] = &ldk.WhisperContentDisambiguationElementText{
			Body:  fmt.Sprintf("# No results for %v, please try another search", text),
			Order: 0,
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

func (l *Loop) GetVenddySubcategoryNames(results []VenddyResult) []VenddyResult {
	venddySubcategories := Venddy{}
	resp, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    "https://venddy.com/api/1.1/obj/subcategory",
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Subcategory GET failed", err)
	}

	err = json.Unmarshal(resp.Data, &venddySubcategories)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}

	venddySubcategories2 := Venddy{}
	resp2, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    "https://venddy.com/api/1.1/obj/subcategory?cursor=100",
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Subcategory GET failed", err)
	}

	err = json.Unmarshal(resp2.Data, &venddySubcategories2)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}

	for i := range venddySubcategories.Response.Results {
		for in := range results {
			for ind := range results[in].Subcategories {
				if venddySubcategories.Response.Results[i].Id == results[in].Subcategories[ind] {
					results[in].SubcategoryNames += "- " + venddySubcategories.Response.Results[i].Name + "\n"
				}
			}
		}
	}

	for i := range venddySubcategories2.Response.Results {
		for in := range results {
			for ind := range results[in].Subcategories {
				if venddySubcategories2.Response.Results[i].Id == results[in].Subcategories[ind] {
					results[in].SubcategoryNames += "- " + venddySubcategories2.Response.Results[i].Name + "\n"
				}
			}
		}
	}

	return results
}

func (l *Loop) GetVenddyTypeNames(results []VenddyResult) []VenddyResult {
	venddyTypes := Venddy{}
	resp, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    "https://venddy.com/api/1.1/obj/solutionType",
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Type GET failed", err)
	}

	err = json.Unmarshal(resp.Data, &venddyTypes)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}

	for i := range venddyTypes.Response.Results {
		for in := range results {
			for ind := range results[in].Types {
				if venddyTypes.Response.Results[i].Id == results[in].Types[ind] {
					results[in].TypeNames += "- " + venddyTypes.Response.Results[i].Name + "\n"
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
	Id               string   `json:"_id"`
	Website          string   `json:"Website"`
	Description      string   `json:"Description"`
	Name             string   `json:"Name"`
	Logo             string   `json:"Logo"`
	Keywords         string   `json:"Search field"`
	Categories       []string `json:"Categories"`
	CategoryNames    string
	Classes          []string `json:"Classes"`
	ClassNames       string
	Subcategories    []string `json:"Subcategories"`
	SubcategoryNames string
	Types            []string `json:"Types"`
	TypeNames        string
	Score            float64 `json:"Score"`
	ReviewCount      float64 `json:"Number of Reviews"`
}

// LoopStop is called by the host when the loop is stopped
func (l *Loop) LoopStop() error {
	l.logger.Info("LoopStop called")
	l.cancel()

	return nil
}

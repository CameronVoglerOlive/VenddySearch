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
			resp, err := GetVenddySearchResults(text, searchLimit, cursor)
			if err != nil {
				log.Fatalln(err)
			}
			defer resp.Body.Close()

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			venddyResponse := VenddyResponse{}

			er := json.Unmarshal(bodyBytes, &venddyResponse)
			if er != nil {
				log.Fatalln(er.Error())
			}

			go func() {
				_, err := l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
					Label:    "Venddy Search",
					Elements: l.CreateDisambiguationElements(venddyResponse.Response, text),
				})
				if err != nil {
					l.logger.Error("failed to emit whisper", "error", err)
				}
			}()
		}()
	})
}

func (l *Loop) CreateDisambiguationElements(response Response, text string) map[string]ldk.WhisperContentDisambiguationElement {
	elements := make(map[string]ldk.WhisperContentDisambiguationElement)

	for i := range response.Results {
		item := response.Results[i]
		elements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentDisambiguationElementOption{
			Label: item.Name,
			Order: uint32(i) + 1,
			OnChange: func(key string) {
				go func() {
					err := l.sidekick.Whisper().List(l.ctx, &ldk.WhisperContentList{
						Label:    item.Name,
						Elements: CreateListElements(item),
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
					resp, err := GetVenddySearchResults(text, searchLimit, cursor)
					if err != nil {
						log.Fatalln(err)
					}
					defer resp.Body.Close()

					bodyBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Fatalln(err)
					}

					venddyResponse := VenddyResponse{}

					er := json.Unmarshal(bodyBytes, &venddyResponse)
					if er != nil {
						log.Fatalln(er.Error())
					}
					_, _ = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
						Label:    "Venddy Search",
						Elements: l.CreateDisambiguationElements(venddyResponse.Response, text),
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
					resp, err := GetVenddySearchResults(text, searchLimit, cursor)
					if err != nil {
						log.Fatalln(err)
					}
					defer resp.Body.Close()

					bodyBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Fatalln(err)
					}

					venddyResponse := VenddyResponse{}

					er := json.Unmarshal(bodyBytes, &venddyResponse)
					if er != nil {
						log.Fatalln(er.Error())
					}
					_, _ = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
						Label:    "Venddy Search",
						Elements: l.CreateDisambiguationElements(venddyResponse.Response, text),
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

func GetVenddySearchResults(text string, max int, startAt int) (*http.Response, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf(`https://venddy.com/api/1.1/obj/vendor?constraints=[{"key":"searchfield", "constraint_type":"text contains", "value":"%v" }]&sort_field=Score&descending=true&limit=%v&cursor=%v`,
			text, max, startAt), nil)
	if err != nil {
		log.Fatalln(err)
	}
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	return client.Do(req)
}

type VenddyResponse struct {
	Response Response `json:"response"`
}

type Response struct {
	Results   []VenddyResult `json:"results"`
	Cursor    int            `json:"Cursor"`
	Remaining int            `json:"Remaining"`
	Count     int            `json:"Count"`
}

type VenddyResult struct {
	Id          string  `json:"_id"`
	Website     string  `json:"Website"`
	Description string  `json:"Description"`
	Name        string  `json:"Name"`
	Logo        string  `json:"Logo"`
	Keywords    string  `json:"Search field"`
	Score       float64 `json:"Score"`
	ReviewCount float64 `json:"Number of Reviews"`
}

// LoopStop is called by the host when the loop is stopped
func (l *Loop) LoopStop() error {
	l.logger.Info("LoopStop called")
	l.cancel()

	return nil
}

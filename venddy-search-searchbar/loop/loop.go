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
var searchLimit = 5
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
			req, err := http.NewRequest("GET",
				fmt.Sprintf(`https://venddy.com/api/1.1/obj/vendor?constraints=[{"key":"searchfield", "constraint_type":"text contains", "value":"%v" }]&sort_field=Score&descending=true&limit=%v&cursor=%v`,
					text, searchLimit, cursor), nil)
			if err != nil {
				log.Fatalln(err)
			}
			q := req.URL.Query()
			req.URL.RawQuery = q.Encode()

			resp, err := client.Do(req)
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

			listElements := make(map[string]ldk.WhisperContentListElement)

			if len(venddyResponse.Response.Results) > 0 {
				for i := range venddyResponse.Response.Results {
					isExtra := i != 0
					listElements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentListElementLink{
						Extra: isExtra,
						Align: ldk.WhisperContentListElementAlignLeft,
						Href:  venddyResponse.Response.Results[i].Website,
						Order: uint32(i),
						Style: ldk.WhisperContentListElementStyleNone,
						Text:  venddyResponse.Response.Results[i].Name,
					}

					listElements[fmt.Sprintf("a_%v", i)] = &ldk.WhisperContentListElementPair{
						Label: "Rating",
						Order: uint32(i),
						Extra: isExtra,
						Value: fmt.Sprintf("%v", venddyResponse.Response.Results[i].Score),
					}

					listElements[fmt.Sprintf("b_%v", i)] = &ldk.WhisperContentListElementPair{
						Label: "Reivews",
						Order: uint32(i),
						Extra: isExtra,
						Value: fmt.Sprintf("%v", venddyResponse.Response.Results[i].ReviewCount),
					}

					listElements[fmt.Sprintf("c_%v", i)] = &ldk.WhisperContentListElementMessage{
						Style:  ldk.WhisperContentListElementStyleNone,
						Header: venddyResponse.Response.Results[i].Keywords,
						Body:   venddyResponse.Response.Results[i].Description,
						Align:  ldk.WhisperContentListElementAlignLeft,
						Order:  uint32(i),
						Extra:  isExtra,
					}

					listElements[fmt.Sprintf("d_%v", i)] = &ldk.WhisperContentListElementDivider{
						Order: uint32(i),
						Extra: isExtra,
					}
				}

				err = l.sidekick.Whisper().List(l.ctx, &ldk.WhisperContentList{
					Label:    fmt.Sprintf("Venddy Search Results for %v", text),
					Elements: listElements,
				})
			} else {
				err = l.sidekick.Whisper().Markdown(l.ctx, &ldk.WhisperContentMarkdown{
					Label:    fmt.Sprintf("Venddy Search Results for %v", text),
					Markdown: fmt.Sprintf("%v didn't return any results, try another seach", text),
				})
			}

			if err != nil {
				l.logger.Error("failed to emit whisper", "error", err)
				return
			}
		}()
	})
}

type VenddyResponse struct {
	Response Response `json:"response"`
}

type Response struct {
	Results   []VenddyResults `json:"results"`
	Cursor    int             `json:"Cursor"`
	Remaining int             `json:"Remaining"`
	Count     int             `json:"Count"`
}

type VenddyResults struct {
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

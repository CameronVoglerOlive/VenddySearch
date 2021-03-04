package loop

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	ldk "github.com/open-olive/loop-development-kit/ldk/go"
)

// Serve creates the new loop and tells the LDK to serve it
func Serve() error {
	logger := ldk.NewLogger("example-search-searchbar")
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

// LoopStart is called by the host when the loop is started to provide access to the host process
func (l *Loop) LoopStart(sidekick ldk.Sidekick) error {
	l.logger.Info("starting example loop")
	l.ctx, l.cancel = context.WithCancel(context.Background())

	l.sidekick = sidekick

	return sidekick.UI().ListenSearchbar(l.ctx, func(text string, err error) {
		l.logger.Info("loop callback called")
		if err != nil {
			l.logger.Error("received error from callback", err)
			return
		}

		go func() {
			req, err := http.NewRequest("GET", `https://venddy.com/api/1.1/obj/vendor?constraints=[{"key":"searchfield", "constraint_type":"text contains", "value":"`+text+`"}]&limit=5`, nil)
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

			// errJson := json.Unmarshal(bodyBytes, &dataMap)
			// var displayError string
			// if errJson != nil {
			// 	displayError = errJson.Error()
			// } else {
			// 	displayError = "no errors"
			// }
			// response, ok := dataMap["response"].(map[string]interface{})["results"]
			// fmt.Println(response)
			// bodyString := string(bodyBytes)
			// "\r\n Name: " + venddyResponse.Response.Results[0].Name +
			// 		"\r\n Description: " + venddyResponse.Response.Results[0].Description +
			// 		"\r\n Website: " + venddyResponse.Response.Results[0].Website +
			// 		"\r\n Keywords: " + venddyResponse.Response.Results[0].Keywords
			// template := "Name: " + venddyResponse.Response.Results[0].Name +
			// 	" Description: " + venddyResponse.Response.Results[0].Description +
			// 	" Website: " + venddyResponse.Response.Results[0].Website +
			// 	" Keywords: " + venddyResponse.Response.Results[0].Keywords
			// imgResp, err := http.NewRequest("GET", venddyResponse.Response.Results[0].Logo, nil)
			// if err != nil {
			// 	log.Fatalln(err)
			// }
			// defer imgResp.Body.Close()
			// file, err := os.Create("/tmp/venImag.png")
			// if err != nil {
			// 	log.Fatalln(err)
			// }
			// defer file.Close()
			// _, err = io.Copy(file, imgResp.Body)
			// if err != nil {
			// 	log.Fatalln(err)
			// }

			// var test map[string]ldk.WhisperContentListElement

			// testStr := ""

			// for i, val := range venddyResponse.Response.Results {
			// 	testStr += string(i)
			// 	test[string(i)+"link"] = &ldk.WhisperContentListElementLink{
			// 		Align: ldk.WhisperContentListElementAlignLeft,
			// 		Href:  val.Website,
			// 		Order: uint32(i),
			// 		Style: ldk.WhisperContentListElementStyleNone,
			// 		Text:  val.Name,
			// 	}

			// 	test[string(i)+"topMessage"] = &ldk.WhisperContentListElementMessage{
			// 		Style:  ldk.WhisperContentListElementStyleNone,
			// 		Header: val.Keywords,
			// 		Body:   val.Description,
			// 		Align:  ldk.WhisperContentListElementAlignLeft,
			// 		Order:  uint32(i + 1),
			// 	}

			// 	test[string(i)+"sectionDivider"] = &ldk.WhisperContentListElementDivider{
			// 		Order: uint32(i + 2),
			// 	}
			// }

			testing := map[string]ldk.WhisperContentListElement{
				"link": &ldk.WhisperContentListElementLink{
					Align: ldk.WhisperContentListElementAlignLeft,
					Href:  venddyResponse.Response.Results[0].Website,
					Order: 0,
					Style: ldk.WhisperContentListElementStyleNone,
					Text:  venddyResponse.Response.Results[0].Name,
				},
				"topMessage": &ldk.WhisperContentListElementMessage{
					Style:  ldk.WhisperContentListElementStyleNone,
					Header: venddyResponse.Response.Results[0].Keywords,
					Body:   venddyResponse.Response.Results[0].Description,
					Align:  ldk.WhisperContentListElementAlignLeft,
					Order:  1,
				},
				"sectionDivider": &ldk.WhisperContentListElementDivider{
					Order: 2,
				},
				"link2": &ldk.WhisperContentListElementLink{
					Align: ldk.WhisperContentListElementAlignLeft,
					Href:  venddyResponse.Response.Results[1].Website,
					Order: 3,
					Extra: true,
					Style: ldk.WhisperContentListElementStyleNone,
					Text:  venddyResponse.Response.Results[1].Name,
				},
				"topMessage2": &ldk.WhisperContentListElementMessage{
					Style:  ldk.WhisperContentListElementStyleNone,
					Header: venddyResponse.Response.Results[1].Keywords,
					Body:   venddyResponse.Response.Results[1].Description,
					Align:  ldk.WhisperContentListElementAlignLeft,
					Extra:  true,
					Order:  4,
				},
				"sectionDivider2": &ldk.WhisperContentListElementDivider{
					Extra: true,
					Order: 5,
				},
				"link3": &ldk.WhisperContentListElementLink{
					Align: ldk.WhisperContentListElementAlignLeft,
					Href:  venddyResponse.Response.Results[2].Website,
					Order: 6,
					Extra: true,
					Style: ldk.WhisperContentListElementStyleNone,
					Text:  venddyResponse.Response.Results[2].Name,
				},
				"topMessage3": &ldk.WhisperContentListElementMessage{
					Style:  ldk.WhisperContentListElementStyleNone,
					Header: venddyResponse.Response.Results[2].Keywords,
					Body:   venddyResponse.Response.Results[2].Description,
					Align:  ldk.WhisperContentListElementAlignLeft,
					Extra:  true,
					Order:  7,
				},
				"sectionDivider3": &ldk.WhisperContentListElementDivider{
					Extra: true,
					Order: 8,
				},
				"link4": &ldk.WhisperContentListElementLink{
					Align: ldk.WhisperContentListElementAlignLeft,
					Href:  venddyResponse.Response.Results[3].Website,
					Order: 9,
					Extra: true,
					Style: ldk.WhisperContentListElementStyleNone,
					Text:  venddyResponse.Response.Results[3].Name,
				},
				"topMessage4": &ldk.WhisperContentListElementMessage{
					Style:  ldk.WhisperContentListElementStyleNone,
					Header: venddyResponse.Response.Results[3].Keywords,
					Body:   venddyResponse.Response.Results[3].Description,
					Align:  ldk.WhisperContentListElementAlignLeft,
					Extra:  true,
					Order:  10,
				},
				"sectionDivider4": &ldk.WhisperContentListElementDivider{
					Extra: true,
					Order: 11,
				},
				"link5": &ldk.WhisperContentListElementLink{
					Align: ldk.WhisperContentListElementAlignLeft,
					Href:  venddyResponse.Response.Results[4].Website,
					Order: 12,
					Extra: true,
					Style: ldk.WhisperContentListElementStyleNone,
					Text:  venddyResponse.Response.Results[4].Name,
				},
				"topMessage5": &ldk.WhisperContentListElementMessage{
					Style:  ldk.WhisperContentListElementStyleNone,
					Header: venddyResponse.Response.Results[4].Keywords,
					Body:   venddyResponse.Response.Results[4].Description,
					Align:  ldk.WhisperContentListElementAlignLeft,
					Extra:  true,
					Order:  13,
				},
			}

			err = l.sidekick.Whisper().List(l.ctx, &ldk.WhisperContentList{
				Label:    "Venddy Search Results for " + text,
				Elements: testing,
			})
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
	Results []VenddyResults `json:Results`
}

type VenddyResults struct {
	Website     string `json:"Website"`
	Description string `json:"Description"`
	Name        string `json:"Name"`
	Logo        string `json:"Logo"`
	Keywords    string `json:"Search field"`
}

// type VenddyResponse struct {
// 	response struct {
// 		results []struct {
// 			Website    string `json:"Website"`
// 			Descrition string `json:"Description"`
// 			Name       string `json:"Name"`
// 			Logo       string `json:"Logo"`
// 		} `json:"results"`
// 	} `json:"response"`
// }

// LoopStop is called by the host when the loop is stopped
func (l *Loop) LoopStop() error {
	l.logger.Info("LoopStop called")
	l.cancel()

	return nil
}

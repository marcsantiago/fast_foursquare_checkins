package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	// pass in your foursquare token
	oauthToken = ""
)

var (
	getRevenuesURI = "https://api.foursquare.com/v2/venues/search?v=20131016&limit=50&near=%s&oauth_token=%s"
	checkinURI     = "https://api.foursquare.com/v2/checkins/add?v=20131016&venueId=%s&oauth_token=%s"
	venueLimit     = 90
)

type VenueSearch struct {
	Meta struct {
		Code      int    `json:"code"`
		RequestID string `json:"requestId"`
	} `json:"meta"`
	Notifications []struct {
		Type string `json:"type"`
		Item struct {
			UnreadCount int `json:"unreadCount"`
		} `json:"item"`
	} `json:"notifications"`
	Response struct {
		Venues    []Venue `json:"venues"`
		Confident bool    `json:"confident"`
		Geocode   struct {
			What    string `json:"what"`
			Where   string `json:"where"`
			Feature struct {
				Cc              string `json:"cc"`
				Name            string `json:"name"`
				DisplayName     string `json:"displayName"`
				MatchedName     string `json:"matchedName"`
				HighlightedName string `json:"highlightedName"`
				WoeType         int    `json:"woeType"`
				ID              string `json:"id"`
				LongID          string `json:"longId"`
				Geometry        struct {
					Center struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"center"`
				} `json:"geometry"`
			} `json:"feature"`
			Parents []interface{} `json:"parents"`
		} `json:"geocode"`
	} `json:"response"`
}

type Venue struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Contact struct {
	} `json:"contact"`
	Location struct {
		Lat            float64 `json:"lat"`
		Lng            float64 `json:"lng"`
		LabeledLatLngs []struct {
			Label string  `json:"label"`
			Lat   float64 `json:"lat"`
			Lng   float64 `json:"lng"`
		} `json:"labeledLatLngs"`
		Cc               string   `json:"cc"`
		Country          string   `json:"country"`
		FormattedAddress []string `json:"formattedAddress"`
	} `json:"location"`
	Categories []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PluralName string `json:"pluralName"`
		ShortName  string `json:"shortName"`
		Icon       struct {
			Prefix string `json:"prefix"`
			Suffix string `json:"suffix"`
		} `json:"icon"`
		Primary bool `json:"primary"`
	} `json:"categories"`
	Verified bool `json:"verified"`
	Stats    struct {
		CheckinsCount int `json:"checkinsCount"`
		UsersCount    int `json:"usersCount"`
		TipCount      int `json:"tipCount"`
	} `json:"stats"`
	VenueRatingBlacklisted bool `json:"venueRatingBlacklisted,omitempty"`
	BeenHere               struct {
		LastCheckinExpiredAt int `json:"lastCheckinExpiredAt"`
	} `json:"beenHere"`
	Specials struct {
		Count int           `json:"count"`
		Items []interface{} `json:"items"`
	} `json:"specials"`
	HereNow struct {
		Count   int           `json:"count"`
		Summary string        `json:"summary"`
		Groups  []interface{} `json:"groups"`
	} `json:"hereNow"`
	ReferralID       string        `json:"referralId"`
	VenueChains      []interface{} `json:"venueChains"`
	HasPerk          bool          `json:"hasPerk"`
	AllowMenuURLEdit bool          `json:"allowMenuUrlEdit,omitempty"`
	URL              string        `json:"url,omitempty"`
}

type Venues []Venue

type Limiter struct {
	Requests chan *http.Request
	Rate     time.Duration
	Throttle <-chan time.Time
	Checkins int
}

func (l *Limiter) checkin() {
	client := &http.Client{Timeout: 5 * time.Second}
	for res := range l.Requests {
		log.Println("waiting for throttle")
		<-l.Throttle
		log.Println("making vender request")
		res, err := client.Do(res)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()
		log.Println("loading json")
		b, _ := ioutil.ReadAll(res.Body)
		var vs VenueSearch
		json.Unmarshal(b, &vs)

		log.Printf("iterating on %d venues\n", len(vs.Response.Venues))
		for _, v := range vs.Response.Venues {
			log.Println("waiting for throttle number 2")
			<-l.Throttle
			uri := fmt.Sprintf(checkinURI, v.ID, oauthToken)
			log.Printf("CHECKING IN %s...\n", v.ID)
			res, err := client.Post(uri, "application/json", nil)
			if err != nil || res.StatusCode != http.StatusOK {
				b, _ := ioutil.ReadAll(res.Body)
				log.Println("ERROR CHECKING IN: ", err, res.StatusCode, string(b))
				continue
			}
			defer res.Body.Close()

			l.Checkins++
			if l.Checkins >= venueLimit {
				return
			}
		}
	}

}

// on verfified accounts we can hit the api 500/hour
// be aware, Four Square blocks the number of checkins one can do a day :-(
func main() {
	file, err := os.Open("./geos/xaa.txt")
	if err != nil {
		log.Fatal(err)
	}

	api := &Limiter{
		Requests: make(chan *http.Request),
		Rate:     time.Hour / 475,
	}
	api.Throttle = time.Tick(api.Rate)

	go api.checkin()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		uri := fmt.Sprintf(getRevenuesURI, url.QueryEscape(scanner.Text()), oauthToken)
		req, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			log.Fatal(err)
		}
		api.Requests <- req
	}
	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	}
	close(api.Requests)

}

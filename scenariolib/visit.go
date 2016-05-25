package scenariolib

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	ua "github.com/coveo/go-coveo/analytics"
	"github.com/coveo/go-coveo/search"
)

// Visit        The struct visit is used to store one visit to the site.
// SearchClient The http client to send search queries
// UAClient     The http client to send the usage analytics data
// LastQuery    The last query that was searched
// LastResponse The last response that was received
// Username     The name of the user visiting
// OriginLevel1 Where the events originate from
// OriginLevel2 Same as OriginLevel1
// LastTab      The tab the user last visited
type Visit struct {
	SearchClient search.Client
	UAClient     ua.Client
	LastQuery    *search.Query
	LastResponse *search.Response
	Username     string
	OriginLevel1 string
	OriginLevel2 string
	LastTab      string
	Config       *Config
	IP           string
}

const (
	// JSUIVERSION Change this to the version of JSUI you want to appear to be using.
	JSUIVERSION string = "0.0.0.0;0.0.0.0"
	// TIMEBETWEENACTIONS The time in seconds to wait between the different actions inside a visit
	TIMEBETWEENACTIONS int = 5
	// ORIGINALL The origin level of All
	ORIGINALL string = "ALL"
)

// NewVisit     Creates a new visit to the search page
// _searchtoken The token used to be able to search
// _uatoken     The token used to send usage analytics events
// _useragent   The user agent the analytics events will see
func NewVisit(_searchtoken string, _uatoken string, _useragent string, c *Config) (*Visit, error) {

	InitLogger(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	v := Visit{}
	v.Username = buildUserEmail(c)
	Info.Printf("New visit from %s", v.Username)
	Info.Printf("On device %s", _useragent)

	// Create the http searchClient
	searchConfig := search.Config{Token: _searchtoken, UserAgent: _useragent, Endpoint: c.SearchEndpoint}
	searchClient, err := search.NewClient(searchConfig)
	if err != nil {
		return nil, err
	}
	v.SearchClient = searchClient

	// Create the http UAClient
	ip := c.RandomIPs[rand.Intn(len(c.RandomIPs))]
	v.IP = ip
	uaConfig := ua.Config{Token: _uatoken, UserAgent: _useragent, IP: ip, Endpoint: c.AnalyticsEndpoint}
	uaClient, err := ua.NewClient(uaConfig)
	if err != nil {
		return nil, err
	}
	v.UAClient = uaClient

	v.Config = c

	return &v, nil
}

func buildUserEmail(c *Config) string {
	return fmt.Sprint(c.FirstNames[rand.Intn(len(c.FirstNames))], ".", c.LastNames[rand.Intn(len(c.LastNames))], c.Emails[rand.Intn(len(c.Emails))])
}

// ExecuteScenario Execute a specific scenario, send the config for all the
// potential random we need to do.
func (v *Visit) ExecuteScenario(scenario Scenario, c *Config) error {
	Info.Printf("Executing scenario named : %s", scenario.Name)
	for i := 0; i < len(scenario.Events); i++ {
		jsonEvent := scenario.Events[i]
		event, err := ParseEvent(&jsonEvent, c)
		if err != nil {
			return err
		}

		err = event.Execute(v)
		if err != nil {
			return err
		}

		WaitBetweenActions()
	}
	return nil
}

func (v *Visit) sendSearchEvent(q, actionCause, actionType string, customData map[string]interface{}) error {
	Info.Printf("Sending Search Event with %v results", v.LastResponse.TotalCount)
	se, err := ua.NewSearchEvent()
	if err != nil {
		return err
	}

	se.Username = v.Username
	se.SearchQueryUID = v.LastResponse.SearchUID
	se.QueryText = q
	se.AdvancedQuery = v.LastQuery.AQ
	se.ActionCause = actionCause
	se.ActionType = actionType
	se.OriginLevel1 = v.OriginLevel1
	se.OriginLevel2 = v.OriginLevel2
	se.NumberOfResults = v.LastResponse.TotalCount
	se.ResponseTime = v.LastResponse.Duration
	if customData != nil {
		se.CustomData = customData
		se.CustomData["JSUIVersion"] = JSUIVERSION
		se.CustomData["ipadress"] = v.IP
	} else {
		se.CustomData = map[string]interface{}{
			"JSUIVersion": JSUIVERSION,
			"ipadress":    v.IP,
		}
	}

	if v.LastResponse.TotalCount > 0 {
		if urihash, ok := v.LastResponse.Results[0].Raw["sysurihash"].(string); ok {
			se.Results = []ua.ResultHash{
				ua.ResultHash{DocumentURI: v.LastResponse.Results[0].URI, DocumentURIHash: urihash},
			}
		} else {
			return errors.New("Cannot convert sysurihash to string in search event")
		}
	}

	// Send a UA search event
	error := v.UAClient.SendSearchEvent(se)
	if error != nil {
		return err
	}
	return nil
}

func (v *Visit) sendViewEvent(pageTitle, pageReferrer, pageURI string) error {
	Info.Printf("Sending PageView Event on URI: %s", pageURI)

	ve := ua.NewViewEvent()

	ve.Username = v.Username
	ve.OriginLevel1 = v.OriginLevel1
	ve.OriginLevel2 = v.OriginLevel2
	ve.Anonymous = false
	ve.PageReferrer = pageReferrer
	ve.PageTitle = pageTitle
	ve.PageURI = pageURI
	ve.CustomData = map[string]interface{}{
		"JSUIVersion": JSUIVERSION,
		"ipadress":    v.IP,
	}

	// Send a UA search event
	err := v.UAClient.SendViewEvent(ve)
	return err
}

func (v *Visit) sendCustomEvent(actionCause, actionType string, customData map[string]interface{}) error {
	Info.Printf("CustomEvent cause: %s ||| type: %s ||| customData: %v", actionCause, actionType, customData)
	ce, err := ua.NewCustomEvent()
	if err != nil {
		return err
	}

	ce.Username = v.Username
	ce.ActionCause = actionCause
	ce.ActionType = actionType
	ce.EventType = actionType
	ce.EventValue = actionCause
	ce.CustomData = customData
	ce.OriginLevel1 = v.OriginLevel1
	ce.OriginLevel2 = v.OriginLevel2
	ce.CustomData["JSUIVersion"] = JSUIVERSION
	ce.CustomData["ipadress"] = v.IP

	// Send a UA search event
	err = v.UAClient.SendCustomEvent(ce)
	return err
}

func (v *Visit) sendClickEvent(rank int, quickview bool) error {
	Info.Printf("Sending ClickEvent rank=%d (quickview %v)", rank+1, quickview)
	event, err := ua.NewClickEvent()
	if err != nil {
		return err
	}

	event.DocumentURI = v.LastResponse.Results[rank].URI
	event.SearchQueryUID = v.LastResponse.SearchUID
	event.DocumentPosition = rank + 1
	if quickview {
		event.ActionCause = "documentQuickview"
		event.ViewMethod = "documentQuickview"
	} else {
		event.ActionCause = "documentOpen"
	}

	event.DocumentTitle = v.LastResponse.Results[rank].Title
	event.QueryPipeline = v.LastResponse.Pipeline
	event.DocumentURL = v.LastResponse.Results[rank].ClickUri
	event.Username = v.Username
	event.OriginLevel1 = v.OriginLevel1
	event.OriginLevel2 = v.OriginLevel2
	if urihash, ok := v.LastResponse.Results[rank].Raw["sysurihash"].(string); ok {
		event.DocumentURIHash = urihash
	} else {
		return errors.New("Cannot convert sysurihash to string")
	}
	if collection, ok := v.LastResponse.Results[rank].Raw["syscollection"].(string); ok {
		event.CollectionName = collection
	} else {
		return errors.New("Cannot convert syscollection to string")
	}
	if source, ok := v.LastResponse.Results[rank].Raw["syssource"].(string); ok {
		event.SourceName = source
	} else {
		return errors.New("Cannot convert syssource to string")
	}

	event.CustomData = map[string]interface{}{
		"JSUIVersion": JSUIVERSION,
		"ipadress":    v.IP,
	}

	err = v.UAClient.SendClickEvent(event)
	if err != nil {
		return err
	}
	return nil
}

func (v *Visit) sendInterfaceChangeEvent(actionCause, actionType string, customData map[string]interface{}) error {
	ice, err := ua.NewSearchEvent()
	if err != nil {
		return err
	}

	ice.Username = v.Username
	ice.SearchQueryUID = v.LastResponse.SearchUID
	ice.QueryText = v.LastQuery.Q
	ice.AdvancedQuery = v.LastQuery.AQ
	ice.ActionCause = actionCause
	ice.ActionType = actionType
	ice.OriginLevel1 = v.OriginLevel1
	ice.OriginLevel2 = v.OriginLevel2
	ice.NumberOfResults = v.LastResponse.TotalCount
	ice.ResponseTime = v.LastResponse.Duration
	ice.CustomData = customData

	if v.LastResponse.TotalCount > 0 {
		if urihash, ok := v.LastResponse.Results[0].Raw["sysurihash"].(string); ok {
			ice.Results = []ua.ResultHash{
				ua.ResultHash{DocumentURI: v.LastResponse.Results[0].URI, DocumentURIHash: urihash},
			}
		} else {
			return errors.New("Cannot convert sysurihash to string in interfaceChange event")
		}
	}

	ice.CustomData = map[string]interface{}{
		"JSUIVersion": JSUIVERSION,
		"ipadress":    v.IP,
	}

	err = v.UAClient.SendSearchEvent(ice)
	if err != nil {
		return err
	}
	return nil
}

// FindDocumentRankByTitle Looks through the last response to a query to find a document
// rank by his title
func (v *Visit) FindDocumentRankByTitle(toFind string) int {
	for i := 0; i < len(v.LastResponse.Results); i++ {
		if strings.Contains(strings.ToLower(v.LastResponse.Results[i].Title), strings.ToLower(toFind)) {
			return i
		}
	}
	return -1
}

// WaitBetweenActions Wait a random number of seconds between user actions
func WaitBetweenActions() {
	time.Sleep(time.Duration(rand.Intn(TIMEBETWEENACTIONS)+3) * time.Second)
}

// Min Function to return the minimal value between two integers, because Go "forgot"
// to code it...
func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetupNTO Function to instanciate with specific values for NTO demo queries
func (v *Visit) SetupNTO() {
	gbs := []*search.GroupByRequest{}
	q := &search.Query{
		Q:               "",
		CQ:              "",
		AQ:              "NOT @objecttype==(User,Case,CollaborationGroup) AND NOT @sysfiletype==(Folder, YouTubePlaylist, YouTubePlaylistItem)",
		NumberOfResults: 20,
		FirstResult:     0,
		Tab:             "All",
		GroupByRequests: gbs,
	}

	if v.Config.PartialMatch {
		q.PartialMatch = v.Config.PartialMatch
		q.PartialMatchKeywords = v.Config.PartialMatchKeywords
		q.PartialMatchThreshold = v.Config.PartialMatchThreshold
	}

	if v.Config.Pipeline != "" {
		q.Pipeline = v.Config.Pipeline
	}

	v.LastQuery = q

	v.OriginLevel1 = "Community"
	v.OriginLevel2 = ORIGINALL
}

// SetupGeneral Function to instanciate with non-specific values
func (v *Visit) SetupGeneral() {
	gbs := []*search.GroupByRequest{}
	q := &search.Query{
		Q:               "",
		CQ:              "",
		AQ:              "",
		NumberOfResults: 20,
		FirstResult:     0,
		Tab:             "All",
		GroupByRequests: gbs,
	}

	if v.Config.PartialMatch {
		q.PartialMatch = v.Config.PartialMatch
		q.PartialMatchKeywords = v.Config.PartialMatchKeywords
		q.PartialMatchThreshold = v.Config.PartialMatchThreshold
	}

	v.LastQuery = q

	v.OriginLevel1 = ORIGINALL
	v.OriginLevel2 = ORIGINALL
}

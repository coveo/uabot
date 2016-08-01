package scenariolib

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	ua "github.com/coveo/go-coveo/analytics"
	"github.com/coveo/go-coveo/search"
	"github.com/coveo/uabot/defaults"
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
	SearchClient       search.Client
	UAClient           ua.Client
	LastQuery          *search.Query
	LastResponse       *search.Response
	Username           string
	OriginLevel1       string
	OriginLevel2       string
	OriginLevel3       string
	LastTab            string
	Config             *Config
	IP                 string
	Anonymous          bool
	Language           string
	WaitBetweenActions bool
}

const (
	// JSUIVERSION Change this to the version of JSUI you want to appear to be using.
	JSUIVERSION string = "0.0.0.0;0.0.0.0"
	// DEFAULTTIMEBETWEENACTIONS The time in seconds to wait between the different actions inside a visit
	DEFAULTTIMEBETWEENACTIONS int = 5
	// ORIGINALL The origin level of All
	ORIGINALL string = "ALL"
)

// NewVisit     Creates a new visit to the search page
// _searchtoken The token used to be able to search
// _uatoken     The token used to send usage analytics events
// _useragent   The user agent the analytics events will see
func NewVisit(_searchtoken string, _uatoken string, _useragent string, language string, c *Config) (*Visit, error) {

	InitLogger(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	v := Visit{}
	v.Config = c

	v.WaitBetweenActions = !c.DontWaitBetweenVisits
	v.Anonymous = false
	if c.AllowAnonymous {
		var threshold float64
		if c.AnonymousThreshold > 0 {
			threshold = c.AnonymousThreshold
		} else {
			threshold = DEFAULTANONYMOUSTHRESHOLD
		}
		if rand.Float64() <= threshold {
			v.Anonymous = true
			Info.Printf("Anonymous visit")
		}
	}
	if !v.Anonymous {
		v.Username = buildUserEmail(c)
		Info.Printf("New visit from %s", v.Username)
	}
	//Info.Printf("On device %s", _useragent)
	if language != "" {
		v.Language = language
	} else {
		if len(v.Config.Languages) > 0 {
			v.Language = v.Config.Languages[rand.Intn(len(v.Config.Languages))]
		} else {
			v.Language = "en"
		}
	}
	Info.Printf("Language of visit : %s", v.Language)

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
		if v.WaitBetweenActions {
			var timeToWait int
			if c.TimeBetweenActions > 0 {
				timeToWait = c.TimeBetweenActions
			} else {
				timeToWait = DEFAULTTIMEBETWEENACTIONS
			}
			WaitBetweenActions(timeToWait)
		}
	}
	return nil
}

func (v *Visit) sendSearchEvent(q, actionCause, actionType string, customData map[string]interface{}) error {
	Info.Printf("Sending Search Event with %v results", v.LastResponse.TotalCount)
	event, err := ua.NewSearchEvent()
	if err != nil {
		return err
	}

	event.Username = v.Username
	event.Anonymous = v.Anonymous
	event.Language = v.Language
	event.SearchQueryUID = v.LastResponse.SearchUID
	event.QueryText = q
	event.AdvancedQuery = v.LastQuery.AQ
	event.ActionCause = actionCause
	event.ActionType = actionType
	event.OriginLevel1 = v.OriginLevel1
	event.OriginLevel2 = v.OriginLevel2
	event.NumberOfResults = v.LastResponse.TotalCount
	event.ResponseTime = v.LastResponse.Duration

	event.CustomData = make(map[string]interface{})

	event.CustomData["JSUIVersion"] = JSUIVERSION
	event.CustomData["ipadress"] = v.IP
	if customData != nil {
		event.CustomData = customData
		event.CustomData["JSUIVersion"] = JSUIVERSION
		event.CustomData["ipadress"] = v.IP
	} else {
		event.CustomData = map[string]interface{}{
			"JSUIVersion": JSUIVERSION,
			"ipadress":    v.IP,
		}
	}

	if v.Config.AllowEntitlements { // Custom shit for besttech, I don't like it
		event.CustomData["entitlement"] = generateEntitlementBesttech(v.Anonymous)
	}

	// Send all the possible random custom data that can be added from the config
	// scenario file.
	for _, elem := range v.Config.RandomCustomData {
		event.CustomData[elem.APIName] = elem.Values[rand.Intn(len(elem.Values))]
	}

	// Override possible values of customData with the specific customData sent
	for k, v := range customData {
		event.CustomData[k] = v
	}

	if v.LastResponse.TotalCount > 0 {
		if urihash, ok := v.LastResponse.Results[0].Raw["sysurihash"].(string); ok {
			event.Results = []ua.ResultHash{
				ua.ResultHash{DocumentURI: v.LastResponse.Results[0].URI, DocumentURIHash: urihash},
			}
		} else {
			return errors.New("Cannot convert sysurihash to string in search event")
		}
	}

	// Send a UA search event
	error := v.UAClient.SendSearchEvent(event)
	if error != nil {
		return err
	}
	return nil
}

func (v *Visit) sendViewEvent(rank int, contentType string) error {
	Info.Printf("Sending ViewEvent rank=%d ", rank+1)

	event, err := ua.NewViewEvent()
	if err != nil {
		return err
	}

	event.PageURI = v.LastResponse.Results[rank].ClickUri
	event.PageTitle = v.LastResponse.Results[rank].Title
	event.ContentType = contentType
	event.Username = v.Username
	event.Anonymous = v.Anonymous
	event.Language = v.Language
	event.OriginLevel1 = v.OriginLevel1
	event.OriginLevel2 = v.OriginLevel2
	event.ContentIdKey = "@sysurihash"
	event.PageReferrer = "Referrer"
	if urihash, ok := v.LastResponse.Results[rank].Raw["sysurihash"].(string); ok {
		event.ContentIdValue = urihash
	} else {
		return errors.New("Cannot convert sysurihash to string")
	}

	// Send a UA view event
	err = v.UAClient.SendViewEvent(event)
	if err != nil {
		return err
	}
	return nil
}

func (v *Visit) sendCustomEvent(actionCause, actionType string, customData map[string]interface{}) error {
	Info.Printf("Sending CustomEvent cause: %s ||| type: %s", actionCause, actionType)
	event, err := ua.NewCustomEvent()
	if err != nil {
		return err
	}

	event.Username = v.Username
	event.Anonymous = v.Anonymous
	event.Language = v.Language
	event.ActionCause = actionCause
	event.ActionType = actionType
	event.EventType = actionType
	event.EventValue = actionCause
	event.CustomData = customData
	event.OriginLevel1 = v.OriginLevel1
	event.OriginLevel2 = v.OriginLevel2
	if customData != nil {
		event.CustomData = customData
		event.CustomData["JSUIVersion"] = JSUIVERSION
		event.CustomData["ipadress"] = v.IP
	} else {
		event.CustomData = map[string]interface{}{
			"JSUIVersion": JSUIVERSION,
			"ipadress":    v.IP,
		}
	}

	if v.Config.AllowEntitlements { // Custom shit for besttech, I don't like it
		event.CustomData["entitlement"] = generateEntitlementBesttech(v.Anonymous)
	}

	// Send all the possible random custom data that can be added from the config
	// scenario file.
	for _, elem := range v.Config.RandomCustomData {
		event.CustomData[elem.APIName] = elem.Values[rand.Intn(len(elem.Values))]
	}

	// Override possible values of customData with the specific customData sent
	for k, v := range customData {
		event.CustomData[k] = v
	}

	// Send a UA search event
	err = v.UAClient.SendCustomEvent(event)
	return err
}

func (v *Visit) sendClickEvent(rank int, quickview bool, customData map[string]interface{}) error {
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
	event.Anonymous = v.Anonymous
	event.Language = v.Language
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

	if v.Config.AllowEntitlements { // Custom shit for besttech, I don't like it
		event.CustomData["entitlement"] = generateEntitlementBesttech(v.Anonymous)
	}

	event.CustomData["author"] = generateRandomAuthor(event.DocumentTitle)

	// Send all the possible random custom data that can be added from the config
	// scenario file.
	for _, elem := range v.Config.RandomCustomData {
		event.CustomData[elem.APIName] = elem.Values[rand.Intn(len(elem.Values))]
	}

	// Override possible values of customData with the specific customData sent
	for k, v := range customData {
		event.CustomData[k] = v
	}

	err = v.UAClient.SendClickEvent(event)
	if err != nil {
		return err
	}
	return nil
}

func (v *Visit) sendInterfaceChangeEvent(actionCause, actionType string, customData map[string]interface{}) error {
	event, err := ua.NewSearchEvent()
	if err != nil {
		return err
	}

	event.Username = v.Username
	event.Anonymous = v.Anonymous
	event.Language = v.Language
	event.SearchQueryUID = v.LastResponse.SearchUID
	event.QueryText = v.LastQuery.Q
	event.AdvancedQuery = v.LastQuery.AQ
	event.ActionCause = actionCause
	event.ActionType = actionType
	event.OriginLevel1 = v.OriginLevel1
	event.OriginLevel2 = v.OriginLevel2
	event.NumberOfResults = v.LastResponse.TotalCount
	event.ResponseTime = v.LastResponse.Duration
	event.CustomData = customData

	if v.LastResponse.TotalCount > 0 {
		if urihash, ok := v.LastResponse.Results[0].Raw["sysurihash"].(string); ok {
			event.Results = []ua.ResultHash{
				ua.ResultHash{DocumentURI: v.LastResponse.Results[0].URI, DocumentURIHash: urihash},
			}
		} else {
			return errors.New("Cannot convert sysurihash to string in interfaceChange event")
		}
	}

	event.CustomData = map[string]interface{}{
		"JSUIVersion": JSUIVERSION,
		"ipadress":    v.IP,
	}

	if v.Config.AllowEntitlements { // Custom shit for besttech, I don't like it
		event.CustomData["entitlement"] = generateEntitlementBesttech(v.Anonymous)
	}

	// Send all the possible random custom data that can be added from the config
	// scenario file.
	for _, elem := range v.Config.RandomCustomData {
		event.CustomData[elem.APIName] = elem.Values[rand.Intn(len(elem.Values))]
	}

	// Override possible values of customData with the specific customData sent
	for k, v := range customData {
		event.CustomData[k] = v
	}

	err = v.UAClient.SendSearchEvent(event)
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
func WaitBetweenActions(timeToWait int) {
	time.Sleep(time.Duration(rand.Intn(timeToWait)) * time.Second)
}

// Min Function to return the minimal value between two integers, because Go "forgot"
// to code it...
func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func generateRandomAuthor(title string) string {
	return defaults.AUTHORNAMES[(int)(math.Mod((float64)(hash(title)), (float64)(len(defaults.AUTHORNAMES))))]
}

func generateEntitlementBesttech(isAnonymous bool) string {
	if isAnonymous {
		return "Anonymous"
	}
	if rand.Float64() <= 0.1 {
		return "Premier"
	}
	return "Basic"
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

	if v.Config.Pipeline != "" {
		q.Pipeline = v.Config.Pipeline
	}

	v.LastQuery = q

	if v.Config.DefaultOriginLevel1 != "" {
		v.OriginLevel1 = v.Config.DefaultOriginLevel1
	} else {
		v.OriginLevel1 = ORIGINALL
	}
	v.OriginLevel2 = ORIGINALL
}

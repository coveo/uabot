package refactor

// BotConfig All the information necessary to run the ua bot
type BotConfig struct {
	OrgName               string              `json:"orgName"`
	Queries               QueriesDataSet      `json:"queriesDataSet"`
	Scenarios             []*Scenario         `json:"scenarios"`
	SearchEndpoint        string              `json:"searchEndpoint,omitempty"`
	AnalyticsEndpoint     string              `json:"analyticsEndpoint,omitempty"`
	Users                 UserDataSet         `json:"userDataSet,omitempty"`
	TimeBetweenVisits     int                 `json:"timeBetweenVisits,omitempty"`
	TimeBetweenActions    int                 `json:"timeBetweenActions,omitempty"`
	AllowAnonymous        bool                `json:"allowAnonymousVisits,omitempty"`
	AnonymousTreshold     float64             `json:"anonymousTreshold,omitempty"`
	AllowEntitlements     bool                `json:"allowEntitlements,omitempty"`
	RandomCustomData      []*RandomCustomData `json:"randomCustomData,omitempty"`
	RandomDocumentAuthors []string            `json:"randomAuthors,omitempty"`
	ScenarioMap           ScenarioMap
}

type QueryParams struct {
	PartialMatch          bool   `json:"partialMatch,omitempty"`
	PartialMatchKeywords  int    `json:"partialMatchKeywords,omitempty"`
	PartialMatchThreshold string `json:"partialMatchThreshold,omitempty"`
	Pipeline              string `json:"pipeline,omitempty"`
	DefaultOriginLevel1   string `json:"defaultOriginLevel1,omitempty"`
	GlobalFilter          string `json:"globalfilter,omitempty"`
}

// QueriesDataSet The dataset of random queries that the bot can use
type QueriesDataSet struct {
	GoodQueries []string `json:"goodQueries"`
	BadQueries  []string `json:"badQueries"`
}

// UserDataSet The dataset of random user information the bot can use
type UserDataSet struct {
	Emails           []string `json:"emailSuffixes,omitempty"`
	FirstNames       []string `json:"firstNames,omitempty"`
	LastNames        []string `json:"lastNames,omitempty"`
	RandomIPs        []string `json:"randomIPs,omitempty"`
	UserAgents       []string `json:"useragents,omitempty"`
	MobileUserAgents []string `json:"mobileuseragents, omitempty"`
	Languages        []string `json:"languages,omitempty"`
}

type RandomCustomData struct {
	APIName string   `json:"apiname"`
	Values  []string `json:"values"`
}

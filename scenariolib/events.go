// Package scenariolib handles everything need to execute a scenario and send all
// information to the usage analytics endpoint
package scenariolib

import "errors"

// ParseEvent A factory to create the correct event type coming from the JSON parse
// of the scenario definition.
func ParseEvent(e *JSONEvent, c *Config) (Event, error) {
	switch e.Type {

	case "Search":
		event, err := newSearchEvent(e, c)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "FakeSearch":
		event, err := newFakeSearchEvent(e, c)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "Click":
		event, err := newClickEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "SearchAndClick":
		event, err := newSearchAndClickEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "TabChange":
		event, err := newTabChangeEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "FacetChange":
		event, err := newFacetEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil
	case "Custom":
		event, err := newCustomEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "View":
		event, err := newViewEvent(e, c)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "SetOrigin":
		event, err := newSetOriginEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil

	case "SetReferrer":
		event, err := newSetReferrerEvent(e)
		if err != nil {
			return nil, err
		}
		return event, nil
	}
	return nil, errors.New("Event type not supported")
}

// Event Generic interface for abstract type Event. All specific event types must
// define the Execute function
type Event interface {
	Execute(v *Visit) error
}

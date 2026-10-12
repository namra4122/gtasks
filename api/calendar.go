package api

import (
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
)

// GetCalendars returns all calendars in the user's calendar list
func GetCalendars(srv *calendar.Service) ([]*calendar.CalendarListEntry, error) {
	var all []*calendar.CalendarListEntry
	pageToken := ""

	for {
		call := srv.CalendarList.List()
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve calendars: %v", wrapScopeError(err))
		}

		all = append(all, r.Items...)

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}

	return all, nil
}

// GetEvents retrieves events in [timeMin, timeMax), with recurring events expanded
// into single instances and ordered by start time.
// If maxResults is 0, fetches all events with pagination.
func GetEvents(srv *calendar.Service, calendarID string, timeMin, timeMax time.Time, maxResults int) ([]*calendar.Event, error) {
	var allEvents []*calendar.Event
	pageToken := ""

	pageSize := int64(250)
	if maxResults > 0 && maxResults < 250 {
		pageSize = int64(maxResults)
	}

	for {
		call := srv.Events.List(calendarID).
			SingleEvents(true).
			OrderBy("startTime").
			TimeMin(timeMin.Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			MaxResults(pageSize)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve events: %v", wrapScopeError(err))
		}

		allEvents = append(allEvents, r.Items...)

		if maxResults > 0 && len(allEvents) >= maxResults {
			allEvents = allEvents[:maxResults]
			break
		}

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}

	return allEvents, nil
}

// CreateEvent inserts an event into a calendar
func CreateEvent(srv *calendar.Service, calendarID string, event *calendar.Event) (*calendar.Event, error) {
	r, err := srv.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, wrapScopeError(err)
	}
	return r, nil
}

// DeleteEvent removes an event from a calendar
func DeleteEvent(srv *calendar.Service, calendarID string, eventID string) error {
	return wrapScopeError(srv.Events.Delete(calendarID, eventID).Do())
}

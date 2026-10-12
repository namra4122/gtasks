package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BRO3886/gtasks/api"
	"github.com/BRO3886/gtasks/internal/config"
	"github.com/BRO3886/gtasks/internal/utils"
	"github.com/araddon/dateparse"
	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"google.golang.org/api/calendar/v3"
)

// calendarCmd represents the calendar command
var calendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "View calendars and view, create, and delete events",
	Long: `
	View calendars and manage events in Google Calendar
	for the currently signed in account.
	Usage:
	gtasks calendar list
	gtasks calendar events [-c "<calendar name|id>"] view|add|rm

	Without -c, the calendar from GTASKS_DEFAULT_CALENDAR or
	calendar.default_calendar in the config file is used,
	falling back to your primary calendar.
	`,
}

var listCalendarsCmd = &cobra.Command{
	Use:   "list",
	Short: "List calendars",
	Long:  `List the calendars in the calendar list of the currently signed in account`,
	Run: func(cmd *cobra.Command, args []string) {
		srv, err := api.GetCalendarService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}

		cals, err := api.GetCalendars(srv)
		if err != nil {
			utils.ErrorP("Error: %v\n", err)
		}

		if listCalendarsFlags.format == "json" {
			type calendarOutput struct {
				Number  int    `json:"number"`
				Name    string `json:"name"`
				ID      string `json:"id"`
				Primary bool   `json:"primary"`
				Access  string `json:"access_role"`
			}
			var output []calendarOutput
			for ind, c := range cals {
				output = append(output, calendarOutput{ind + 1, c.Summary, c.Id, c.Primary, c.AccessRole})
			}
			writeJSON(output)
			return
		}

		for ind, c := range cals {
			primary := ""
			if c.Primary {
				primary = " (primary)"
			}
			utils.Print("[%d] %s%s\n", ind+1, c.Summary, primary)
		}
	},
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "View, create, and delete events in a calendar",
	Long: `
	View, create, and delete events in a calendar.
	Usage:
	gtasks calendar events [-c "<calendar name|id>"] view|add|rm
	`,
}

var viewEventsCmd = &cobra.Command{
	Use:   "view",
	Short: "View events in a calendar",
	Long: `
	View events in a calendar. Defaults to today's events.
	Pick a range with --today, --tomorrow, --week, or --from/--to.
	You can control output with --format: table (default), json, csv.

	Examples:
	gtasks calendar events view
	gtasks calendar events view --week --format json
	gtasks calendar events view --from 2026-10-01 --to 2026-10-31
	`,
	Run: func(cmd *cobra.Command, args []string) {
		srv, err := api.GetCalendarService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}
		calID, calName := resolveCalendar(srv)

		from, to, err := viewEventsFlags.window.resolve()
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		events, err := api.GetEvents(srv, calID, from, to, viewEventsFlags.max)
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		switch viewEventsFlags.format {
		case "json":
			writeJSON(eventOutputs(events, calName))
		case "csv":
			outputEventsCSV(events)
		default:
			if len(events) == 0 {
				utils.Warn("No events in %s between %s and %s\n", calName, from.Format("02 Jan 2006"), to.Add(-time.Second).Format("02 Jan 2006"))
				return
			}
			outputEventsTable(events, calName)
		}
	},
}

var createEventCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an event to a calendar",
	Long: `
	Add an event to a calendar.

	Start/end accept dates like '2026-10-13 15:00', 'Oct 13 2026 3:30pm',
	and the words 'today' / 'tomorrow' (e.g. 'tomorrow 15:00').
	A start without a time creates an all-day event.
	The end defaults to start + --duration (1h), or one day for all-day events.

	Examples:
	gtasks calendar events add -t "Standup" -s "tomorrow 09:30" --duration 15m
	gtasks calendar events add -t "Offsite" -s 2026-10-20 -e 2026-10-21 --all-day
	`,
	Run: func(cmd *cobra.Command, args []string) {
		if addEventFlags.title == "" || addEventFlags.start == "" {
			utils.ErrorP("Please specify a title (-t) and a start (-s)\n")
		}

		srv, err := api.GetCalendarService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}
		calID, calName := resolveCalendar(srv)

		event, err := buildEvent()
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		r, err := api.CreateEvent(srv, calID, event)
		if err != nil {
			utils.ErrorP("Unable to create event: %v\n", err)
		}
		utils.Info("Event created in %s: %s (%s)\n", calName, r.Summary, formatEventRange(r))
	},
}

var deleteEventCmd = &cobra.Command{
	Use:   "rm [event-number]",
	Short: "Delete an event from a calendar",
	Long: `
	Delete an event from a calendar. Events are numbered as in
	'events view' for the same range flags (default: today).

	Examples:
	gtasks calendar events rm
	gtasks calendar events rm 2 --tomorrow
	`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		srv, err := api.GetCalendarService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}
		calID, calName := resolveCalendar(srv)

		from, to, err := deleteEventFlags.window.resolve()
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		events, err := api.GetEvents(srv, calID, from, to, 0)
		if err != nil {
			utils.ErrorP("%v\n", err)
		}
		if len(events) == 0 {
			utils.ErrorP("No events in %s for the selected range\n", calName)
		}

		ev := events[getEventIndex(args, events)]
		if err := api.DeleteEvent(srv, calID, ev.Id); err != nil {
			utils.ErrorP("Unable to delete event: %v\n", err)
		}
		utils.Info("Event deleted: %s\n", ev.Summary)
	},
}

// timeWindow holds the range flags shared by commands that list events
type timeWindow struct {
	today, tomorrow, week bool
	from, to              string
}

func addWindowFlags(cmd *cobra.Command, w *timeWindow) {
	cmd.Flags().BoolVar(&w.today, "today", false, "today (default)")
	cmd.Flags().BoolVar(&w.tomorrow, "tomorrow", false, "tomorrow")
	cmd.Flags().BoolVar(&w.week, "week", false, "today and the next 6 days")
	cmd.Flags().StringVar(&w.from, "from", "", "start of range (e.g. '2026-10-01')")
	cmd.Flags().StringVar(&w.to, "to", "", "end of range, inclusive when a date (default: one day after --from)")
	cmd.MarkFlagsMutuallyExclusive("today", "tomorrow", "week", "from")
}

// resolve returns the [from, to) window selected by the flags
func (w timeWindow) resolve() (time.Time, time.Time, error) {
	today := startOfDay(time.Now())

	switch {
	case w.tomorrow:
		return today.AddDate(0, 0, 1), today.AddDate(0, 0, 2), nil
	case w.week:
		return today, today.AddDate(0, 0, 7), nil
	case w.from != "":
		from, fromDateOnly, err := parseDateTime(w.from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from: %v", err)
		}
		if w.to == "" {
			if fromDateOnly {
				return from, from.AddDate(0, 0, 1), nil
			}
			return from, from.Add(24 * time.Hour), nil
		}
		to, toDateOnly, err := parseDateTime(w.to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to: %v", err)
		}
		if toDateOnly {
			to = to.AddDate(0, 0, 1) // make the end date inclusive
		}
		if !to.After(from) {
			return time.Time{}, time.Time{}, fmt.Errorf("--to must be after --from")
		}
		return from, to, nil
	case w.to != "":
		return time.Time{}, time.Time{}, fmt.Errorf("--to requires --from")
	default:
		return today, today.AddDate(0, 0, 1), nil
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

var (
	relativeDayRe = regexp.MustCompile(`(?i)^(today|tomorrow)\b\s*`)
	timeOfDayRe   = regexp.MustCompile(`(?i)(:|\d\s*[ap]\.?m\.?\b|T\d)`)
)

// parseDateTime parses a local date/time, supporting the 'today' and 'tomorrow'
// prefixes on top of dateparse. dateOnly reports whether the input had no time.
func parseDateTime(input string) (t time.Time, dateOnly bool, err error) {
	input = strings.TrimSpace(input)
	if m := relativeDayRe.FindStringSubmatch(input); m != nil {
		day := startOfDay(time.Now())
		if strings.EqualFold(m[1], "tomorrow") {
			day = day.AddDate(0, 0, 1)
		}
		input = day.Format("2006-01-02") + " " + input[len(m[0]):]
		input = strings.TrimSpace(input)
	}

	dateOnly = !timeOfDayRe.MatchString(input)

	t, err = dateparse.ParseLocal(input)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("could not parse %q. Valid examples: https://github.com/araddon/dateparse#extended-example", input)
	}
	return t, dateOnly, nil
}

// buildEvent assembles an event from the add command flags
func buildEvent() (*calendar.Event, error) {
	start, startDateOnly, err := parseDateTime(addEventFlags.start)
	if err != nil {
		return nil, fmt.Errorf("invalid start: %v", err)
	}
	allDay := addEventFlags.allDay || startDateOnly

	event := &calendar.Event{
		Summary:     addEventFlags.title,
		Description: addEventFlags.note,
		Location:    addEventFlags.location,
	}

	if allDay {
		// All-day end dates are exclusive in the Calendar API
		end := startOfDay(start).AddDate(0, 0, 1)
		if addEventFlags.end != "" {
			e, _, err := parseDateTime(addEventFlags.end)
			if err != nil {
				return nil, fmt.Errorf("invalid end: %v", err)
			}
			end = startOfDay(e).AddDate(0, 0, 1)
		}
		if !end.After(startOfDay(start)) {
			return nil, fmt.Errorf("end must not be before start")
		}
		event.Start = &calendar.EventDateTime{Date: start.Format("2006-01-02")}
		event.End = &calendar.EventDateTime{Date: end.Format("2006-01-02")}
		return event, nil
	}

	end := start.Add(addEventFlags.duration)
	if addEventFlags.end != "" {
		e, endDateOnly, err := parseDateTime(addEventFlags.end)
		if err != nil {
			return nil, fmt.Errorf("invalid end: %v", err)
		}
		if endDateOnly {
			return nil, fmt.Errorf("end needs a time for a timed event (or use --all-day)")
		}
		end = e
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end must be after start")
	}
	event.Start = &calendar.EventDateTime{DateTime: start.Format(time.RFC3339)}
	event.End = &calendar.EventDateTime{DateTime: end.Format(time.RFC3339)}
	return event, nil
}

// resolveCalendar picks the calendar from -c, then config, then primary.
// Returns the calendar ID and a display name.
func resolveCalendar(srv *calendar.Service) (string, string) {
	want := calendarFlag
	if want == "" {
		want = config.GetDefaultCalendar()
	}
	if want == "primary" {
		return "primary", "primary calendar"
	}

	cals, err := api.GetCalendars(srv)
	if err != nil {
		utils.ErrorP("Error: %v\n", err)
	}
	for _, c := range cals {
		if c.Id == want || strings.EqualFold(c.Summary, want) {
			return c.Id, c.Summary
		}
	}
	utils.ErrorP("incorrect calendar name '%s'. Run 'gtasks calendar list' to see calendars\n", want)
	return "", ""
}

func getEventIndex(args []string, events []*calendar.Event) int {
	if len(args) == 1 {
		index, err := strconv.Atoi(args[0])
		if err != nil || index < 1 || index > len(events) {
			utils.ErrorP("Incorrect event number\n")
		}
		return index - 1
	}

	var items []string
	for _, ev := range events {
		items = append(items, fmt.Sprintf("%s  %s", formatEventRange(ev), ev.Summary))
	}
	prompt := promptui.Select{
		Label: "Select Event",
		Items: items,
	}
	option, _, err := prompt.Run()
	if err != nil {
		utils.ErrorP("Error: %s\n", err.Error())
	}
	return option
}

// eventTimes returns the start and end of an event and whether it is all-day
func eventTimes(ev *calendar.Event) (time.Time, time.Time, bool) {
	parse := func(dt *calendar.EventDateTime) (time.Time, bool) {
		if dt == nil {
			return time.Time{}, false
		}
		if dt.DateTime != "" {
			t, _ := time.Parse(time.RFC3339, dt.DateTime)
			return t.Local(), false
		}
		t, _ := time.ParseInLocation("2006-01-02", dt.Date, time.Local)
		return t, true
	}
	start, allDay := parse(ev.Start)
	end, _ := parse(ev.End)
	return start, end, allDay
}

func formatEventRange(ev *calendar.Event) string {
	start, end, allDay := eventTimes(ev)
	if allDay {
		last := end.AddDate(0, 0, -1)
		if !last.After(start) {
			return start.Format("Mon 02 Jan") + ", all day"
		}
		return start.Format("Mon 02 Jan") + " - " + last.Format("Mon 02 Jan") + ", all day"
	}
	if start.YearDay() == end.YearDay() && start.Year() == end.Year() {
		return start.Format("Mon 02 Jan 15:04") + "-" + end.Format("15:04")
	}
	return start.Format("Mon 02 Jan 15:04") + " - " + end.Format("Mon 02 Jan 15:04")
}

type EventOutput struct {
	Number      int    `json:"number"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
	Calendar    string `json:"calendar"`
	Status      string `json:"status,omitempty"`
	Link        string `json:"link,omitempty"`
}

func eventOutputs(events []*calendar.Event, calName string) []EventOutput {
	output := []EventOutput{}
	for ind, ev := range events {
		start, end, allDay := eventTimes(ev)
		layout := time.RFC3339
		if allDay {
			layout = "2006-01-02"
		}
		output = append(output, EventOutput{
			Number:      ind + 1,
			ID:          ev.Id,
			Title:       ev.Summary,
			Start:       start.Format(layout),
			End:         end.Format(layout),
			AllDay:      allDay,
			Location:    ev.Location,
			Description: ev.Description,
			Calendar:    calName,
			Status:      ev.Status,
			Link:        ev.HtmlLink,
		})
	}
	return output
}

func outputEventsTable(events []*calendar.Event, calName string) {
	utils.Print("Events in %s:\n", calName)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"No", "When", "Title", "Location"})
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetCenterSeparator("|")
	table.SetRowLine(false)
	table.SetRowSeparator("-")
	table.SetAutoWrapText(false)

	for ind, ev := range events {
		table.Append([]string{
			fmt.Sprintf("%d", ind+1),
			formatEventRange(ev),
			truncate(ev.Summary, 40),
			truncate(ev.Location, 30),
		})
	}

	table.Render()
}

func outputEventsCSV(events []*calendar.Event) {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	_ = writer.Write([]string{"No", "Start", "End", "All Day", "Title", "Location", "Description"})
	for _, ev := range eventOutputs(events, "") {
		_ = writer.Write([]string{
			fmt.Sprintf("%d", ev.Number),
			ev.Start,
			ev.End,
			strconv.FormatBool(ev.AllDay),
			ev.Title,
			ev.Location,
			ev.Description,
		})
	}
}

func writeJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(v)
}

var calendarFlag string

var (
	listCalendarsFlags struct {
		format string
	}
	viewEventsFlags struct {
		window timeWindow
		max    int
		format string
	}
	addEventFlags struct {
		title, start, end, note, location string
		duration                          time.Duration
		allDay                            bool
	}
	deleteEventFlags struct {
		window timeWindow
	}
)

func init() {
	listCalendarsCmd.Flags().StringVar(&listCalendarsFlags.format, "format", "table", "output format: table, json")

	addWindowFlags(viewEventsCmd, &viewEventsFlags.window)
	viewEventsCmd.Flags().IntVar(&viewEventsFlags.max, "max", 0, "maximum number of events to return (0 = all)")
	viewEventsCmd.Flags().StringVar(&viewEventsFlags.format, "format", "table", "output format: table, json, csv")

	createEventCmd.Flags().StringVarP(&addEventFlags.title, "title", "t", "", "event title")
	createEventCmd.Flags().StringVarP(&addEventFlags.start, "start", "s", "", "start date/time (e.g. 'tomorrow 15:00', '2026-10-20')")
	createEventCmd.Flags().StringVarP(&addEventFlags.end, "end", "e", "", "end date/time (default: start + --duration)")
	createEventCmd.Flags().DurationVar(&addEventFlags.duration, "duration", time.Hour, "event length when --end is not set (e.g. 30m, 1h30m)")
	createEventCmd.Flags().BoolVar(&addEventFlags.allDay, "all-day", false, "create an all-day event")
	createEventCmd.Flags().StringVarP(&addEventFlags.note, "note", "n", "", "event description")
	createEventCmd.Flags().StringVar(&addEventFlags.location, "location", "", "event location")
	createEventCmd.MarkFlagsMutuallyExclusive("end", "duration")

	addWindowFlags(deleteEventCmd, &deleteEventFlags.window)

	eventsCmd.PersistentFlags().StringVarP(&calendarFlag, "calendar", "c", "", "calendar name or id (default: primary)")
	eventsCmd.AddCommand(viewEventsCmd, createEventCmd, deleteEventCmd)
	calendarCmd.AddCommand(listCalendarsCmd, eventsCmd)
	rootCmd.AddCommand(calendarCmd)
}

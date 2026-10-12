package cmd

import (
	"os"
	"time"

	"github.com/BRO3886/gtasks/api"
	"github.com/BRO3886/gtasks/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"google.golang.org/api/tasks/v1"
)

var agendaCmd = &cobra.Command{
	Use:   "agenda",
	Short: "Show calendar events and due tasks together",
	Long: `
	Show calendar events and pending tasks due in the same range.
	Defaults to today. Pick a range with --today, --tomorrow, --week,
	or --from/--to. Tasks come from every tasklist unless -l is set.

	Examples:
	gtasks agenda
	gtasks agenda --week --format json
	gtasks agenda -c "Work" -l "My Tasks" --tomorrow
	`,
	Run: func(cmd *cobra.Command, args []string) {
		from, to, err := agendaFlags.window.resolve()
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		calSrv, err := api.GetCalendarService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}
		calID, calName := resolveCalendar(calSrv)
		events, err := api.GetEvents(calSrv, calID, from, to, 0)
		if err != nil {
			utils.ErrorP("%v\n", err)
		}

		taskSrv, err := api.GetService()
		if err != nil {
			utils.ErrorP("Failed to get service: %v\n", err)
		}
		dueTasks := agendaTasks(taskSrv, from, to)

		if agendaFlags.format == "json" {
			writeJSON(struct {
				From   string             `json:"from"`
				To     string             `json:"to"`
				Events []EventOutput      `json:"events"`
				Tasks  []agendaTaskOutput `json:"tasks"`
			}{from.Format(time.RFC3339), to.Format(time.RFC3339), eventOutputs(events, calName), dueTasks})
			return
		}

		if len(events) == 0 {
			utils.Warn("No events in %s\n", calName)
		} else {
			outputEventsTable(events, calName)
		}
		utils.Print("\n")
		if len(dueTasks) == 0 {
			utils.Warn("No tasks due\n")
			return
		}

		utils.Print("Tasks due:\n")
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Due", "Title", "Tasklist"})
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetBorder(false)
		table.SetCenterSeparator("|")
		table.SetAutoWrapText(false)
		for _, t := range dueTasks {
			table.Append([]string{t.Due, truncate(t.Title, 40), t.Tasklist})
		}
		table.Render()
	},
}

type agendaTaskOutput struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Due         string `json:"due"`
	Tasklist    string `json:"tasklist"`
}

// agendaTasks returns pending tasks due within [from, to) from the -l tasklist,
// or from every tasklist when -l is not set.
func agendaTasks(srv *tasks.Service, from, to time.Time) []agendaTaskOutput {
	lists, err := api.GetTaskLists(srv)
	if err != nil {
		utils.ErrorP("Error: %v\n", err)
	}

	// Task due dates are stored as midnight UTC of the due day
	utcDay := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	dueMin := utcDay(from)
	dueMax := utcDay(to)
	if !startOfDay(to).Equal(to) {
		dueMax = dueMax.AddDate(0, 0, 1)
	}

	output := []agendaTaskOutput{}
	found := false
	for _, l := range lists {
		if agendaFlags.tasklist != "" && l.Title != agendaFlags.tasklist {
			continue
		}
		found = true

		items, err := api.GetTasksDue(srv, l.Id, dueMin, dueMax)
		if err != nil {
			utils.ErrorP("%v\n", err)
		}
		utils.Sort(items, "due")
		for _, t := range items {
			output = append(output, agendaTaskOutput{
				Title:       t.Title,
				Description: t.Notes,
				Due:         dueDay(t.Due),
				Tasklist:    l.Title,
			})
		}
	}
	if !found {
		utils.ErrorP("incorrect task-list name '%s'\n", agendaFlags.tasklist)
	}
	return output
}

// dueDay returns a task's due date as YYYY-MM-DD. The API stores it as
// midnight UTC, so it is read in UTC to avoid shifting the day.
func dueDay(due string) string {
	parsed, err := time.Parse(time.RFC3339, due)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format("2006-01-02")
}

var agendaFlags struct {
	window   timeWindow
	tasklist string
	format   string
}

func init() {
	addWindowFlags(agendaCmd, &agendaFlags.window)
	agendaCmd.Flags().StringVarP(&calendarFlag, "calendar", "c", "", "calendar name or id (default: primary)")
	agendaCmd.Flags().StringVarP(&agendaFlags.tasklist, "tasklist", "l", "", "only include tasks from this tasklist (default: all)")
	agendaCmd.Flags().StringVar(&agendaFlags.format, "format", "table", "output format: table, json")
	rootCmd.AddCommand(agendaCmd)
}

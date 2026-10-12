---
title: "Calendar commands"
description: "View your Google Calendar agenda and add or delete events from the terminal with gtasks, and combine today's events with due tasks in a single agenda view."
draft: false
weight: 4
sitemap:
  priority: 0.8
---

gtasks can read and create Google Calendar events with the same login as tasks.

> Calendar access needs the Google Calendar API enabled in your Google Cloud project, with the
> `calendar.events` and `calendar.calendarlist.readonly` scopes on the OAuth consent screen.
> If you logged in before calendar support, run `gtasks logout && gtasks login` once to grant access.

## Calendars

```bash
gtasks calendar list
gtasks calendar list --format json
```

## Events

All `events` commands take `-c "<calendar name|id>"`. Without it, gtasks uses
`GTASKS_DEFAULT_CALENDAR` or `calendar.default_calendar` from the config file,
falling back to your primary calendar.

### View events

Defaults to today. Pick a range with `--today`, `--tomorrow`, `--week` (today plus 6 days),
or `--from`/`--to` (a date-only `--to` is inclusive).

```bash
gtasks calendar events view
gtasks calendar events view --week --format json
gtasks calendar events view -c "Work" --from 2026-10-01 --to 2026-10-31 --format csv
```

### Add an event

```bash
# Timed event; ends after --duration (default 1h)
gtasks calendar events add -t "Standup" -s "tomorrow 09:30" --duration 15m

# Explicit end, location, and description
gtasks calendar events add -t "Review" -s "2026-10-20 14:00" -e "2026-10-20 15:30" --location "Room 4" -n "Q4 numbers"

# All-day events: a start with no time, or --all-day. --end is the last day, inclusive.
gtasks calendar events add -t "Offsite" -s 2026-10-20 -e 2026-10-21
```

Start and end accept the formats supported by
[dateparse](https://github.com/araddon/dateparse#extended-example), plus the prefixes
`today` and `tomorrow` (e.g. `today 3pm`).

### Delete an event

Events are numbered as in `events view` for the same range flags.

```bash
gtasks calendar events rm            # pick from today's events
gtasks calendar events rm 2 --tomorrow
```

## Agenda

`gtasks agenda` shows events and pending tasks due in the same range. Tasks come from every
tasklist unless `-l` is set. It accepts the same range flags as `events view`.

```bash
gtasks agenda
gtasks agenda --week --format json
gtasks agenda -c "Work" -l "My Tasks" --tomorrow
```

JSON output:

```json
{
  "from": "2026-10-12T00:00:00+05:30",
  "to": "2026-10-13T00:00:00+05:30",
  "events": [
    {
      "number": 1,
      "id": "abc123",
      "title": "Standup",
      "start": "2026-10-12T09:30:00+05:30",
      "end": "2026-10-12T09:45:00+05:30",
      "all_day": false,
      "calendar": "primary calendar",
      "status": "confirmed",
      "link": "https://www.google.com/calendar/event?eid=..."
    }
  ],
  "tasks": [
    { "title": "Write journal", "due": "2026-10-12", "tasklist": "My Tasks" }
  ]
}
```

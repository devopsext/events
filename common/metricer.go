package common

type Counter interface {
	Inc(labelValues ...string) Counter
}

type Metricer interface {
	Counter(name, description string, labels []string) Counter
}

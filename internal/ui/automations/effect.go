package automations

import "time"

// The requests the manager queues for the commands. Each write reads the
// Automations and their records back, so the rail shows the change at once
// without waiting for the next check.

// Listing is the stored Automations and their records as a write left them.
type Listing struct {
	Rows    []Row
	Records []Record
}

func list(services Services) (Listing, error) {
	rows, err := services.Store.AutomationRows()
	if err != nil {
		return Listing{}, err
	}
	var records []Record
	if services.RecordsDir != "" {
		records, _, _ = LoadRecords(services.RecordsDir)
	}
	return Listing{Rows: rows, Records: records}, nil
}

func written(services Services, err error) (Listing, error) {
	if err != nil {
		return Listing{}, err
	}
	return list(services)
}

// SaveRequest stores the dialog's configuration: a new Automation when
// Editing is empty, else a replacement for the one Editing names. Gen is the
// dialog that asked. The dialog builds Config and Tools afresh for each
// save, so the request shares no slice with it.
type SaveRequest struct {
	Gen     uint64
	Editing string
	Config  Config
	Tools   []string
	Now     time.Time
}

func (SaveRequest) EffectName() string { return "Automation save" }

// SaveResult is the configuration as stored and the listing after it.
type SaveResult struct {
	Config Config
	Listing
}

func (r SaveRequest) Run(services Services) (any, error) {
	var stored Config
	var err error
	if r.Editing == "" {
		stored, err = Add(services.Book, r.Config, true, r.Tools, r.Now)
	} else {
		stored, err = Change(services.Book, r.Editing, r.Config, r.Tools, r.Now)
	}
	listing, err := written(services, err)
	return SaveResult{Config: stored, Listing: listing}, err
}

// EnableRequest enables or disables the Automation ID.
type EnableRequest struct {
	ID      string
	Enabled bool
	Now     time.Time
}

func (EnableRequest) EffectName() string { return "Automation enable" }

func (r EnableRequest) Run(services Services) (any, error) {
	return written(services, SetEnabled(services.Book, r.ID, r.Enabled, r.Now))
}

// RemoveRequest deletes the Automation ID.
type RemoveRequest struct{ ID string }

func (RemoveRequest) EffectName() string { return "Automation delete" }

func (r RemoveRequest) Run(services Services) (any, error) {
	return written(services, Remove(services.Book, r.ID))
}

// RunNowRequest asks for the Automation ID to run for the instant Now.
type RunNowRequest struct {
	ID  string
	Now time.Time
}

func (RunNowRequest) EffectName() string { return "Automation run now" }

func (r RunNowRequest) Run(services Services) (any, error) { return RunNow(services, r.ID, r.Now) }

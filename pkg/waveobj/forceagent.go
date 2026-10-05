// SPDX-License-Identifier: Apache-2.0
package waveobj

// ForceAgentInstance is an inert, durable identity. Runtime status is only
// advanced by an explicit operation with evidence from its execution attempt.
type ForceAgentInstance struct {
	OID                      string      `json:"oid"`
	Version                  int         `json:"version"`
	Meta                     MetaMapType `json:"meta"`
	ProjectID                string      `json:"projectid"`
	ProjectVersion           int         `json:"projectversion"`
	ProfileID                string      `json:"profileid"`
	ProfileVersion           int         `json:"profileversion"`
	TabID                    string      `json:"tabid"`
	BlockID                  string      `json:"blockid"`
	TitleSnapshot            string      `json:"titlesnapshot"`
	IconSnapshot             string      `json:"iconsnapshot"`
	PromptSnapshot           string      `json:"promptsnapshot"`
	PromptHash               string      `json:"prompthash"`
	Adapter                  string      `json:"adapter"`
	AdapterVersion           string      `json:"adapterversion"`
	Connection               string      `json:"connection"`
	RootPath                 string      `json:"rootpath"`
	CLIHistoryContext        string      `json:"clihistorycontext,omitempty"`
	ExecutionRoot            string      `json:"executionroot,omitempty"`
	ExecutionLeaseRoot       string      `json:"executionleaseroot,omitempty"`
	ConfirmedCWD             string      `json:"confirmedcwd,omitempty"`
	CWDSource                string      `json:"cwdsource,omitempty"`
	ClaudeSessionID          string      `json:"claudesessionid,omitempty"`
	PriorClaudeSessionID     string      `json:"priorclaudesessionid,omitempty"`
	IdentityEvidence         string      `json:"identityevidence"`
	Generation               int64       `json:"generation"`
	OperationIntent          string      `json:"operationintent,omitempty"`
	OperationRequestKey      string      `json:"operationrequestkey,omitempty"`
	OperationPhase           string      `json:"operationphase,omitempty"`
	AttemptID                string      `json:"attemptid,omitempty"`
	WasLaunched              bool        `json:"waslaunched,omitempty"`
	CurrentSessionLaunched   bool        `json:"currentsessionlaunched,omitempty"`
	LocalPID                 int         `json:"localpid,omitempty"`
	LocalProcessStartTs      int64       `json:"localprocessstartts,omitempty"`
	LocalProcessGroupID      int         `json:"localprocessgroupid,omitempty"`
	LocalProcessGroupStartTs int64       `json:"localprocessgroupstartts,omitempty"`
	LocalBootID              string      `json:"localbootid,omitempty"`
	AttemptStartedAt         int64       `json:"attemptstartedat,omitempty"`
	AttemptFinishedAt        int64       `json:"attemptfinishedat,omitempty"`
	AttemptExitCode          *int        `json:"attemptexitcode,omitempty"`
	PriorAttemptID           string      `json:"priorattemptid,omitempty"`
	JobID                    string      `json:"jobid,omitempty"`
	WriterLeaseKey           string      `json:"writerleasekey,omitempty"`
	Status                   string      `json:"status"`
	ErrorCode                string      `json:"errorcode,omitempty"`
	CreatedAt                int64       `json:"createdat"`
	UpdatedAt                int64       `json:"updatedat"`
}

func (*ForceAgentInstance) GetOType() string               { return OType_ForceAgentInstance }
func (v *ForceAgentInstance) MarshalJSON() ([]byte, error) { return ToJson(v) }

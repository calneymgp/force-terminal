// SPDX-License-Identifier: Apache-2.0
package waveobj

// ForceProject is a saved work destination. A blank connection means local.
type ForceProject struct {
	OID        string      `json:"oid"`
	Version    int         `json:"version"`
	Meta       MetaMapType `json:"meta"`
	Name       string      `json:"name"`
	Icon       string      `json:"icon"`
	Connection string      `json:"connection"`
	RootPath   string      `json:"rootpath"`
	Archived   bool        `json:"archived"`
	CreatedAt  int64       `json:"createdat"`
	UpdatedAt  int64       `json:"updatedat"`
}

func (*ForceProject) GetOType() string { return OType_ForceProject }

func (project *ForceProject) MarshalJSON() ([]byte, error) { return ToJson(project) }

// ForceAgentProfile is reusable; an eventual running agent takes a snapshot.
type ForceAgentProfile struct {
	OID          string      `json:"oid"`
	Version      int         `json:"version"`
	Meta         MetaMapType `json:"meta"`
	Title        string      `json:"title"`
	Icon         string      `json:"icon"`
	SystemPrompt string      `json:"systemprompt"`
	Adapter      string      `json:"adapter"`
	Archived     bool        `json:"archived"`
	CreatedAt    int64       `json:"createdat"`
	UpdatedAt    int64       `json:"updatedat"`
}

func (*ForceAgentProfile) GetOType() string { return OType_ForceAgentProfile }

func (profile *ForceAgentProfile) MarshalJSON() ([]byte, error) { return ToJson(profile) }

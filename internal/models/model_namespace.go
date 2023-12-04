package models

type Namespace struct {

	Infix string `json:"infix,omitempty"`

	Suffix string `json:"suffix,omitempty"`

	// How long should the namespace be preserved until it becomes obsolete and is automatically cleaned up.
	Duration string `json:"duration,omitempty"`

	// A list of users to be authorized as editors in this namespace.
	Users []string `json:"users,omitempty"`
}

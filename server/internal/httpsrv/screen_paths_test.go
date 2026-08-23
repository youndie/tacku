package httpsrv_test

import "github.com/youndie/tacku/server/internal/httpsrv"

// publicScreens are the addresses a walk visits, and the sign-in form is in the list only when this
// build carries it.
//
// It was a constant in four files. That was correct while every build served the same set, and the
// moment a door became a build tag it became four places to remember — the shape of drift this
// repository has met before under a different name.
func screenPaths() []string {
	paths := []string{
		"/screens/catch-up",
		"/screens/board",
		"/forms/my-tasks",
		"/forms/new-task",
		"/forms/new-board",
		"/forms/bulk-move",
	}
	if httpsrv.DoorPresent {
		paths = append(paths, "/forms/sign-in")
	}
	return paths
}

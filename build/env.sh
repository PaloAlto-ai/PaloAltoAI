#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
paadir="$workspace/src/github.com/PaloAltoAi"
if [ ! -L "$paadir/go-PaloAltoAi" ]; then
    mkdir -p "$paadir"
    cd "$paadir"
    ln -s ../../../../../. go-PaloAltoAi
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$paadir/go-PaloAltoAi"
PWD="$paadir/go-PaloAltoAi"

# Launch the arguments with the configured environment.
exec "$@"

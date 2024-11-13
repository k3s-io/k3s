# Easy way for auto adding images to k3s

Date: 2024-10-2

## Status

Proposed

## Context

Since the feature for embedded registry, the users appeared with a question about having to manually import images, specially in edge environments.

As a result, there is a need for a folder who can handle this action, where every image there will be watched by a controller for changes or new images, this new images or new changes will be added to the containerd image store.

The controller will watch the agent/images folder that is the default folder for the images, as the first iteration about the controller he will mainly work with the default image folder, but in the future we can set to watch more folders.

The main idea for the controller is to create a map for the file infos maintaining the state for the files, with that we can see if a file was modified and if the size changed.

### Map to handle the state from the files

This map will have the entire filepath of the file in the `key` value, since we can get the value from the key with only the `event.Name`

```go
    map[string]fs.FileInfo
```

### Why use fsnotify

With this library we can easily use for any linux distros without the need to port for a specify distro and can also run in windows.

The main idea for the watch will be taking care of the last time that was modified the image file.

fsnotify has a great toolset for handling changes in files, since the code will have a channel to receive events such as CREATE, RENAME, REMOVE and WRITE.

### How the controller will work with the events

When the controller receive a event saying that a file was created, he will add to the map and import the images if the event that he has received is not a directory and then import the image.

When the controller receive a event saying that a file was writen, he will verify if the file has the size changed and if the file has the time modified based on the time and size from the state.

When the controller receive a event saying that a file was renamed, or removed, he will delete this file from the state. when a file is renamed, it is created a new file with the same infos but with a the new name, so the watcher will sent for the controller a event saying that a file was created.

## Decision

- Decided

## Consequences

Good:
- Better use of embedded containerd image store.
- Fsnotify it's a indirect dependency that upstream uses

Bad:
- The need for another dependency
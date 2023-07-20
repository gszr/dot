# dot

dot files manager.

## What

* Map dotfiles living in a directory -- say, a git repo -- living somewhere 
  to their destination
* Future: fetch additional resources from the web, also mapping them to their 
  destination

## Usage

Let's start with an example:

```
files:
  i3:
  imwheelrc:
  config/alacritty:
  config/redshift.conf:
```

Behavior

- Top-level `files` map lists files along with mapping attributes
- Each file name maps to a file in the current working directory -- ie,
  `i3` and `imwheelrc` are both files in the CWD where the `dot` CLI was
  executed. Files may list the following optional mapping attributes:
  * `to`: where it ends up
    - If the file starts with a `~`, it is resolved to the current user's home
      dir
    - If omitted, the default is `~/.<file name>`; in the example above,
      `i3` maps to `~/.i3`
  * `as`: how the mapping is performed - can be `symlink` or `copy`, for a symlink and a copy,
    respectively (the default is a symlink)
  * `os`: restricts the OS where the mapping applies; can be `linux`, `macos` or
    `all` - if not specified, `all` is implied

Another example:
```
map:
  i3:
    os: linux
  xinitrc:
    os: linux
  Xresources:
    os: linux
  imwheelrc:
    os: linux
  config/alacritty:
    os: macos
  docker/config.json:
    as: copy
```

## Features

- [x] Map source to inferred destination (`file` to `~/.file`)
  - [x] `.file` to `~/.file`
- [x] Map source to specified destination
  - [x] Resolve tilde in destination
- [x] Verbose mode
- [x] rm-only flag
- [x] `cd` opt (files live under a subdir)
- [x] Create destination path if needed
- [x] OS filter
- [ ] Validate dot file
- [ ] Tests
- [ ] CI/CD

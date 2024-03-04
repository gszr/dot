# dot

dot files manager.

## Download

Download the latest version with:

```
$ curl --remote-name-all --location  $( \
    curl -s https://api.github.com/repos/gszr/dot/releases/latest \
    | grep "browser_download_url.*$(uname -s)-$(uname -m).*" \
    | cut -d : -f 2,3 \
    | tr -d \" )
```

Linux and MacOS on `x86_64` or `arm64`.

## What

* Map dotfiles living in a directory somehwere -- say, a git repo --  
  to their destination
* Future: fetch additional resources from the web, also mapping them to their 
  destination

## Usage

Let's start with an example:

```
map:
  i3:
  imwheelrc:
  config/alacritty.yml:
  config/redshift.conf:
```

### Behavior

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
  * `with`: valid only `as: copy` is used; lists variables whose values are replaced
    in the input file's contents using the [Go templating engine](https://pkg.go.dev/text/template).
    Currently, the feature exposes the only `.Os` variable within `with` values.

### Examples

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
  config/alacritty.yml:
    os: macos
  docker/config.json:
    as: copy

opt:
  cd: dots/
```

In this example, all files live under a subdirectory `dots/`:
```
$ tree .
.
├── dots
│   ├── config
│   │   └── alacritty.yml
│   ├── docker
│   │   └── config.json
│   ├── i3
│   ├── imwheelrc
│   ├── xinitrc
│   └── Xresources
└── dot.yml
```

#### The templating feature

Some system utilities have built-in support for simple variable substitutions through
environment variables, while others do not. In these cases, one can use `dot`'s templating
feature to allow for customizations.

Let's take the following dots spec as an example:

```
map:
  gnupg/gpg-agent.conf:
    as: copy
    with:
      PinentryPrefix: '{{if eq .Os "darwin"}}/opt/homebrew/bin{{else}}/usr/bin{{end}}'
```

The contents of `gnupg/gpg-agent.conf` look like the following:
```
default-cache-ttl 1800
max-cache-ttl 3600
enable-ssh-support
pinentry-program {{.PinentryPrefix}}/pinentry-tty
```

This will result in the correct path to `pinentry-tty` being set during the dot
file mapping process.

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
- [x] CI/CD
- [ ] Validate dot file
- [ ] Tests

# Bindicate

`bindicate` is a WIP bind-mounting utility that creates bind mounts of files
into a directory outside of the FHS standard. It is a supporting utility for
image based OS upgrades to "protect" configuration files from being overwritten
by an upgrade.

## Installation

1. Build the binary:
   ```bash
   go build -o bindicate bin/bindicate.go
   ```

2. Install the binary to system PATH:
   ```bash
   sudo cp bindicate /usr/bin/
   ```

3. Create configuration directory:
   ```bash
   sudo mkdir -p /var/lib/bindicate
   ```

## Configuration

Create a configuration file at `/var/lib/bindicate/config.json`:

```json
{
  "Prefix": "/var/lib/bindicate/protected",
  "Paths": [
    "/etc/hostname",
    "/etc/hosts",
    "/etc/fstab"
  ]
}
```

## Usage

Setup bind mounts:
```bash
sudo bindicate setup-bindmounts
```

Update /etc/fstab with bind mount entries:
```bash
sudo bindicate write-fstab
```

## Dracut Integration (Early Boot)

To run bindicate during initramfs before the OS boots:

1. Install the dracut module:
   ```bash
   sudo cp -r dracut/90bindicate /usr/lib/dracut/modules.d/
   ```

2. Rebuild the initramfs:
   ```bash
   sudo dracut -f
   ```

3. Optional kernel parameters:
   - `rd.bindicate.disable=1` - Disable bindicate in initramfs
   - `rd.bindicate.config=/custom/path/config.json` - Use custom config path
   - `rd.bindicate.update_fstab=1` - Also update fstab during boot

The dracut hook will automatically run `bindicate` during the pre-pivot phase,
ensuring your configuration files are bind-mounted before the main OS starts.

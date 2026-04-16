package config

// AddonConfig contains the static configuration of the Home Assistant addon
// Please note that the addon config is not what you think it is:
// the addon config is a static configuration that is baked into the addon image
// at build time and cannot be changed by the user; in practice it's the config.yaml
// that you can find in the root of the git repository.
// We only read 1 field from that file, i.e. the version of the addon, which just used
// to display it in the UI.
type AddonConfig struct {
	Version string `yaml:"version"`
}

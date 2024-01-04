# Asana Markdown export

This is a small set of scripts I wrote to export my Asana data to Obsidian.
I've made it public in case anyone else wants to use it,
but I'm providing it as-is
(i.e. I'm not providing any level of support for it).

## Usage

[Download Nix](https://nixos.org/download) if you don't already have it.
Generate an [Asana access token](https://app.asana.com/0/my-apps)
then copy it into the `ASANA_ACCESS_TOKEN` environment variable.
Then run the following:

```shell
nix build &&
result/bin/asana-my-tasks > my-tasks.json &&
result/bin/asana-to-md out < my-tasks.json
```

## License

[Apache 2.0](LICENSE)

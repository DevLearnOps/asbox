# Future Work

## Change container name rendering to allow multiple agent sessions per project

At the moment we don't have the option to run multiple sandboxes against the same project because of sandbox container name conflict. We could either:
- change the logic to include a random seed to the sandbox name
- allow users to override the name via cli options
- or maybe both, and only seed randomly if there is name conflict

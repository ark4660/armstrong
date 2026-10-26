Client manifests compiled into the launcher, one per Minecraft version.

Generate or update one with:

    armstrong client resolve -mc 26.3 -o internal/client/manifests/26.3.json

then boot-test it (`armstrong boot-test -mc 26.3`) before committing.

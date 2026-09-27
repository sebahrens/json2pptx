# Private local rendering fonts

Bundled headless LibreOffice on this Mac needs its supplied fontconfig file
explicitly included in a private `FONTCONFIG_FILE`. Use the bundled executable,
not desktop LibreOffice. Give the private configuration a writable cache and
include this repository's `svggen/fonts` directory to expose its bundled faces.
Do not install fonts globally or change template font names to repair a renderer
environment.

If Aptos is unavailable, include `fontconfig-aptos-fallback.conf` in that private
configuration. It keeps an installed original first and explicitly falls back to
Arial. The fallback is not native Aptos fidelity. The observed uncontrolled
fallback was HiraMaruPro-W4, which broke the blue section identifier into two
lines. The same unchanged PPTX rendered joined `01` using ArialMT with this
fallback.

Record the configuration hash, actual PDF fonts, source/template hashes and
reviewed images in the render receipt. Use a fresh preview cache directory after
changing font configuration: the existing source cache key does not include
fontconfig. A prior review does not approve changed pixels. Verify all layouts,
then review only pixel-changed originals; preserve the previous raw reviews.

This configuration applies to visual evidence, not to generation or template
conformance. Local test inventory must still include gitignored `p-style.pptx`.

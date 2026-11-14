# Changelog

## [2.2.0](https://github.com/laluowen/permidex/compare/v2.1.0...v2.2.0) (2026-11-14)


### Features

* **cache:** ~/.cache by default; XDG_CACHE_HOME and PERMIDEX_CACHE_DIR override it ([41abd8c](https://github.com/laluowen/permidex/commit/41abd8c4b0d69a8412c6ccfb789ea3db03a5a735))
* **cache:** PERMIDEX_CACHE_DIR overrides the cache directory ([9c8e4fe](https://github.com/laluowen/permidex/commit/9c8e4fe627f22c1a49756d6bf348da6395acf436))
* **cli:** --cache-dir|-c prints the cache directory and exits ([853d683](https://github.com/laluowen/permidex/commit/853d683aa6145f381ab2722d4549e0638b0967ef))


### Bug Fixes

* **cache:** ~/.cache by default on every platform; honour XDG_CACHE_HOME; absolute PERMIDEX_CACHE_DIR ([06767ce](https://github.com/laluowen/permidex/commit/06767ced2d9b8bc778645969097d3fd1229e92d6))
* **cache:** cache variables must be absolute; relative values are an error ([be1c781](https://github.com/laluowen/permidex/commit/be1c7816611b7753c2e602cdfee51f3245198fe9))
* **cache:** ignore relative XDG_CACHE_HOME per spec; PERMIDEX_CACHE_DIR must be absolute ([09a4045](https://github.com/laluowen/permidex/commit/09a40450ac79dcf0d0a49f738e7b41b9fa06b9aa))
* **cache:** invalid XDG_CACHE_HOME warns and is ignored; invalid PERMIDEX_CACHE_DIR fails everywhere ([8e5ac23](https://github.com/laluowen/permidex/commit/8e5ac23c6600b76dba4d2d98e5f38d89c6904476))
* **cache:** use the user cache directory (os.UserCacheDir) ([560f0d8](https://github.com/laluowen/permidex/commit/560f0d8f808ee4f22a0822d9fd8620656b0bd75c))
* **skill,cli:** check --cache-dir's exit status; show --cache-dir in usage ([4cf9d27](https://github.com/laluowen/permidex/commit/4cf9d27e619b2eb4e4f2094baea95517c4356075))

## [2.1.0](https://github.com/laluowen/permidex/compare/v2.0.0...v2.1.0) (2026-11-13)


### Features

* **browse:** footer carries every key; drop help screen; home/end move to top and bottom ([14c9f9a](https://github.com/laluowen/permidex/commit/14c9f9af8856eecca8f582b82a83388806131591))
* **browse:** footer carries every key; drop the help screen ([eab21d6](https://github.com/laluowen/permidex/commit/eab21d666f4503a580b2877eec50fc3005772b6e))
* **browse:** footer order: move, open, back, select, sections, start screen, done ([2ddcc6c](https://github.com/laluowen/permidex/commit/2ddcc6c0374a13953b5d9a9e7018d3fa5707cee7))

## [2.0.0](https://github.com/laluowen/permidex/compare/v1.0.0...v2.0.0) (2026-11-12)


### ⚠ BREAKING CHANGES

* `permidex` is now `permidex gcp`, and `permidex refresh` is `permidex gcp -r`. The cache moves from ~/.cache/permidex/roles.json to ~/.cache/permidex/gcp/roles.json, so run `permidex gcp -r` once. The module path is now github.com/laluowen/permidex/v2.

### Features

* browse providers through a shared browser; GCP is `permidex gcp` ([67f7b97](https://github.com/laluowen/permidex/commit/67f7b97da1c63b565a496fb1fb57e5ca08b0171e))
* **cli:** accept the provider after flags too (permidex -r gcp) ([891a995](https://github.com/laluowen/permidex/commit/891a9950d70e61437c2b59301bed2a2289d57b2a))
* **gcp:** write the schema beside the cache on refresh; validate output in tests ([1883682](https://github.com/laluowen/permidex/commit/18836827d6bb9d32ce4173f534e43b8d4db54c2e))
* versioned roles cache with JSON schema and agent skill ([5fbb6d0](https://github.com/laluowen/permidex/commit/5fbb6d0172a59bdf7a68cb267c9426e08f94eba3))


### Bug Fixes

* **browse:** name sections next to jump keys in help and the bottom bar; review fixes ([989eeba](https://github.com/laluowen/permidex/commit/989eeba99dcf3fa3c8900ee04307cb1eef7f5575))
* **browse:** section jump keys are ctrl+j, ctrl+k, ctrl+l ([5a41503](https://github.com/laluowen/permidex/commit/5a41503b61f57729dc931be4c0e6289fe4b2b7b8))
* **cli:** fix the test constant that broke the build ([f90a287](https://github.com/laluowen/permidex/commit/f90a287bc51ebf97bce106e7b01711b6890e41cb))

## 1.0.0 (2026-11-12)


### Features

* initial permidex browser ([1a39952](https://github.com/laluowen/permidex/commit/1a399526460f1ba5ad45e9aa07cefb2e76376d7b))
* **tui:** colour headers, section titles and key hints ([a1e6b63](https://github.com/laluowen/permidex/commit/a1e6b63343efadf5bee0ce026847e110f759b557))
* **tui:** colour picked rows green ([5372671](https://github.com/laluowen/permidex/commit/5372671fa75a6ca7884a12fd8478a35742e8b94a))
* **tui:** drill-down navigation replaces pivot keys ([380f696](https://github.com/laluowen/permidex/commit/380f69611c70268df44158a7828260d863449040))
* **tui:** keep services as plain prefixes, include in YAML output ([6508ded](https://github.com/laluowen/permidex/commit/6508ded8905d68c03d399e32ee4e15c8addb8c75))
* **tui:** per-view search placeholder and scoping tests ([9031398](https://github.com/laluowen/permidex/commit/9031398d18a568f8bde12fd27c6554f503c64ca2))
* **tui:** search titles, grouped results, blank start, hint bar and help ([819f091](https://github.com/laluowen/permidex/commit/819f0918dedaaa06ac57d6980ff36a64d44fb422))
* **tui:** tab selection, YAML output on ctrl+q, jump keys, FQDN service names ([105195a](https://github.com/laluowen/permidex/commit/105195a2011c5f97baae9828c06ef67f877a9755))


### Bug Fixes

* TUI to stderr so stdout is only the YAML; untrack code-review.md ([3a1d1af](https://github.com/laluowen/permidex/commit/3a1d1af229e539c4e0ee490777c89dd7cdd8ff1a))
* **tui:** drop unused reverse style ([f4fff7a](https://github.com/laluowen/permidex/commit/f4fff7a6a8c86a4116224c934a2e8ab227dffdd5))
* **tui:** enter/esc navigation, ctrl+c quit, breadcrumb line ([737ba5d](https://github.com/laluowen/permidex/commit/737ba5dbb4f91d27898896ed4c0377a81e89e235))
* **tui:** help jump-key order and aligned key column ([11bc650](https://github.com/laluowen/permidex/commit/11bc650aaeb372e6e4810baac14f862578d7cee1))
* **tui:** roles, permissions, services order in every view ([e88ba7d](https://github.com/laluowen/permidex/commit/e88ba7d30c80431a9e21cd522d51f0c8484dc13c))
* **tui:** YAML sections in roles, permissions, services order ([7f0904c](https://github.com/laluowen/permidex/commit/7f0904c44265d843a1b7d73e5ec6d7fdfcf1c3ed))

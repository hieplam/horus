# Changelog

## 0.1.0 (2026-11-16)


### Features

* **backend:** the comfy-cli backend installs ComfyUI on macOS ([#101](https://github.com/hieplam/horus/issues/101)) ([#113](https://github.com/hieplam/horus/issues/113)) ([bd4409c](https://github.com/hieplam/horus/commit/bd4409c5193032acdccc0ae7086bb29d540087d8))
* **backend:** the comfy-cli backend starts and stops ComfyUI on macOS ([#102](https://github.com/hieplam/horus/issues/102)) ([#116](https://github.com/hieplam/horus/issues/116)) ([88ea73a](https://github.com/hieplam/horus/commit/88ea73a5f501045db668e2545c5ea310a7e1fa73))
* **cli:** add horus doctor and horus setup, and finish the three layers ([#63](https://github.com/hieplam/horus/issues/63)) ([#78](https://github.com/hieplam/horus/issues/78)) ([999a4c3](https://github.com/hieplam/horus/commit/999a4c34c921387aeedaaeed1b417bf102360fd8))
* **cli:** horus comfy-mcp, and one .mcp.json for Windows and macOS ([#105](https://github.com/hieplam/horus/issues/105)) ([#118](https://github.com/hieplam/horus/issues/118)) ([b7b71cf](https://github.com/hieplam/horus/commit/b7b71cff0d976608af3f08ceeac412ec5b9333b9))
* **doctor:** a WARN line for the macOS privacy folders, and the address of HORUS_COMFYUI_URL ([#103](https://github.com/hieplam/horus/issues/103)) ([#115](https://github.com/hieplam/horus/issues/115)) ([250302f](https://github.com/hieplam/horus/commit/250302fc76205beb4b85cd46baeccc9bef184a4b))
* **install:** the install scripts and their CI jobs ([#139](https://github.com/hieplam/horus/issues/139)) ([#155](https://github.com/hieplam/horus/issues/155)) ([cc1a4e9](https://github.com/hieplam/horus/commit/cc1a4e91b58faa71e193780f649b8b3997bdf92c))
* **mcp:** install_workflow runs the setup plan after accept_setup ([#141](https://github.com/hieplam/horus/issues/141)) ([#158](https://github.com/hieplam/horus/issues/158)) ([2b2c54f](https://github.com/hieplam/horus/commit/2b2c54f62a1c751c0f3dc5757a5604f01ace5782))
* **register:** horus register -client claude|codex ([#142](https://github.com/hieplam/horus/issues/142)) ([#156](https://github.com/hieplam/horus/issues/156)) ([c41630e](https://github.com/hieplam/horus/commit/c41630e303ec6e56e2309d3fc8076d1e1fb9f000))
* **release:** horus version, release build and release-please ([#137](https://github.com/hieplam/horus/issues/137)) ([#151](https://github.com/hieplam/horus/issues/151)) ([32414fb](https://github.com/hieplam/horus/commit/32414fb3cb95b97024b9d1b38da3c77cee1e05b4))
* **settings:** a machine with only the program lists the built-in workflows ([#138](https://github.com/hieplam/horus/issues/138)) ([#152](https://github.com/hieplam/horus/issues/152)) ([d807449](https://github.com/hieplam/horus/commit/d807449d785e3a81d91ac90b2c1631f06e26d5e1))
* **setup:** the whole setup plan with package steps and -with-packages ([#140](https://github.com/hieplam/horus/issues/140)) ([#153](https://github.com/hieplam/horus/issues/153)) ([6dee6ca](https://github.com/hieplam/horus/commit/6dee6ca60c425c32b5b420b5534e3511d68bba57))
* **tools:** goal tool for Phase 4 and the baseline ([#136](https://github.com/hieplam/horus/issues/136)) ([#149](https://github.com/hieplam/horus/issues/149)) ([594b970](https://github.com/hieplam/horus/commit/594b970851dcdedb1dc40221252ff392d3d387ff))
* **uninstall:** horus uninstall ([#143](https://github.com/hieplam/horus/issues/143)) ([#160](https://github.com/hieplam/horus/issues/160)) ([aade064](https://github.com/hieplam/horus/commit/aade06467311d97a20d52def2c24698381f1af1c))


### Bug Fixes

* **docs:** lint-public finds nothing, and CI runs it ([#157](https://github.com/hieplam/horus/issues/157)) ([#159](https://github.com/hieplam/horus/issues/159)) ([1297e09](https://github.com/hieplam/horus/commit/1297e0956d685da1e72152b0697b03eda982f196))
* **doctor:** the handler runs the model check on the setup service ([#86](https://github.com/hieplam/horus/issues/86)) ([#94](https://github.com/hieplam/horus/issues/94)) ([64e93a6](https://github.com/hieplam/horus/commit/64e93a651ae0f26219eb0690132e836076c03383))
* **mcpserver:** describe_workflow uses the install_state rule of provision ([#88](https://github.com/hieplam/horus/issues/88)) ([#95](https://github.com/hieplam/horus/issues/95)) ([e8cfd6b](https://github.com/hieplam/horus/commit/e8cfd6ba3fcb8284674547f3332e19a1598ec896))
* **provision:** map the folder of a model file that is already present ([#126](https://github.com/hieplam/horus/issues/126)) ([1585e74](https://github.com/hieplam/horus/commit/1585e74999b66d04715816491cbef51717c77d8e))
* **tools:** the archtest rows need the three archtest tests to pass ([#80](https://github.com/hieplam/horus/issues/80)) ([#83](https://github.com/hieplam/horus/issues/83)) ([6789c49](https://github.com/hieplam/horus/commit/6789c49e64da14579a06daf1b8566f89de56a8c9))
* **tools:** the CI part reads the parity and go jobs ([#81](https://github.com/hieplam/horus/issues/81)) ([#84](https://github.com/hieplam/horus/issues/84)) ([9699bb8](https://github.com/hieplam/horus/commit/9699bb8d4a8fe4fb4df21e8b9d8f7fe0355a03f8))

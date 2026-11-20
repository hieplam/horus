# Changelog

## [0.1.4](https://github.com/hieplam/horus/compare/v0.1.3...v0.1.4) (2026-11-20)


### Features

* **mcpserver:** MCP over HTTP at /mcp with the token, the header checks and register -http ([#204](https://github.com/hieplam/horus/issues/204)) ([#247](https://github.com/hieplam/horus/issues/247)) ([d38888a](https://github.com/hieplam/horus/commit/d38888aa1080033aef55514df946db9a7b3a55ba))

## [0.1.3](https://github.com/hieplam/horus/compare/v0.1.2...v0.1.3) (2026-11-20)


### Features

* **mcpserver:** write caller, resource_key and loaded_resource_key ([#232](https://github.com/hieplam/horus/issues/232)) ([#241](https://github.com/hieplam/horus/issues/241)) ([ede62f4](https://github.com/hieplam/horus/commit/ede62f406c62dbb18911566e347b0b7c7e0b04e5))

## [0.1.2](https://github.com/hieplam/horus/compare/v0.1.1...v0.1.2) (2026-11-20)


### Features

* **catalog:** the kind of a workflow in the manifest and the tools ([#184](https://github.com/hieplam/horus/issues/184)) ([#208](https://github.com/hieplam/horus/issues/208)) ([e190106](https://github.com/hieplam/horus/commit/e190106baf3e002ce110ae8b177d9b030e5c2049))
* **jobs:** the job data, the jobs service and the broker contract suite ([#198](https://github.com/hieplam/horus/issues/198)) ([#213](https://github.com/hieplam/horus/issues/213)) ([193af6b](https://github.com/hieplam/horus/commit/193af6b7fd3c2d62eea888f13195c1b922c94500))
* **mcpserver:** generate_image through the job queue in the stack ([#202](https://github.com/hieplam/horus/issues/202)) ([#240](https://github.com/hieplam/horus/issues/240)) ([58b8d57](https://github.com/hieplam/horus/commit/58b8d57f06b18926141d90380dca35009401d703))
* **postgres:** the job store and the Postgres broker ([#199](https://github.com/hieplam/horus/issues/199)) ([#218](https://github.com/hieplam/horus/issues/218)) ([778401f](https://github.com/hieplam/horus/commit/778401fcc446785c6ef9a598d5ab62ce76e96105))
* **provision:** install a speech workflow: the setup plan, install_workflow, horus setup and horus doctor ([#188](https://github.com/hieplam/horus/issues/188)) ([#221](https://github.com/hieplam/horus/issues/221)) ([87399da](https://github.com/hieplam/horus/commit/87399dac1d92c2f3ee50b4cad54938ae209339a9))
* **provision:** uninstall a speech workflow: uninstall_workflow, horus uninstall and the oracle of speech-install ([#189](https://github.com/hieplam/horus/issues/189)) ([#225](https://github.com/hieplam/horus/issues/225)) ([91abd7a](https://github.com/hieplam/horus/commit/91abd7a35facaf34e7423a13b0362f85f3dab8c0))
* **speak:** the speak service and the tool generate_speech ([#187](https://github.com/hieplam/horus/issues/187)) ([#215](https://github.com/hieplam/horus/issues/215)) ([bf94290](https://github.com/hieplam/horus/commit/bf94290287f347ea0c3e05a0024a73409b0e4b34))
* **speechworker:** the speech worker and its Go adapter ([#186](https://github.com/hieplam/horus/issues/186)) ([#211](https://github.com/hieplam/horus/issues/211)) ([dd15786](https://github.com/hieplam/horus/commit/dd157865a10b16fd5b1225355efd710b5ac01e35))
* **stack:** compose.yaml, horus api, horus ready and NOT_READY ([#201](https://github.com/hieplam/horus/issues/201)) ([#226](https://github.com/hieplam/horus/issues/226)) ([d4a6796](https://github.com/hieplam/horus/commit/d4a6796583b761d807ec1ddde2eb97380a586f4d))
* **webui:** the Horus UI, a read-only page of horus api ([#205](https://github.com/hieplam/horus/issues/205)) ([#234](https://github.com/hieplam/horus/issues/234)) ([99bd780](https://github.com/hieplam/horus/commit/99bd78067adb26254f2c1b64bcc2014e20167de1))
* **worker:** horus worker, the one worker of the job queue ([#200](https://github.com/hieplam/horus/issues/200)) ([#220](https://github.com/hieplam/horus/issues/220)) ([c5a2a47](https://github.com/hieplam/horus/commit/c5a2a47bc062cc4758548c5bc094e0cad19ddb0c))
* **workflows:** add the speech workflow vi-speech ([#185](https://github.com/hieplam/horus/issues/185)) ([#212](https://github.com/hieplam/horus/issues/212)) ([cff691e](https://github.com/hieplam/horus/commit/cff691e674ddf76a30ab53e2391618479446f02b))


### Bug Fixes

* **jobs:** the worker stops a running job at a cancel request ([#219](https://github.com/hieplam/horus/issues/219)) ([#222](https://github.com/hieplam/horus/issues/222)) ([1cb30a4](https://github.com/hieplam/horus/commit/1cb30a4a6f65789e1a078de6614f548a51cc34c5))
* **platform:** RemoveSelf returns when its PowerShell is ready ([#196](https://github.com/hieplam/horus/issues/196)) ([#243](https://github.com/hieplam/horus/issues/243)) ([4e29446](https://github.com/hieplam/horus/commit/4e294469b1bf7c79030408af6744f58407295727))
* **postgres:** the stub claim test starts both claims by an event, not a 0.5 s window ([#227](https://github.com/hieplam/horus/issues/227)) ([#228](https://github.com/hieplam/horus/issues/228)) ([afde9ea](https://github.com/hieplam/horus/commit/afde9ea418ac147319476517438769b06d528f06))
* **provision:** the install state of a speech workflow reads the speech folder ([#214](https://github.com/hieplam/horus/issues/214)) ([#217](https://github.com/hieplam/horus/issues/217)) ([7942985](https://github.com/hieplam/horus/commit/794298502307acb403429eaba32f6cff713a53f2))
* **speech:** read the transcript after 0.5 s of silence, and set the limit of speech-real from 240 measured files ([#223](https://github.com/hieplam/horus/issues/223)) ([#230](https://github.com/hieplam/horus/issues/230)) ([e7de5b3](https://github.com/hieplam/horus/commit/e7de5b3caed1e75acf4eaf3e3de7368a58f59605))
* **vi-speech:** say in text.rules how to count the words of the share ([#237](https://github.com/hieplam/horus/issues/237)) ([#244](https://github.com/hieplam/horus/issues/244)) ([d250ca3](https://github.com/hieplam/horus/commit/d250ca32120492c528703ba7197270067bf5feb5))
* **worker:** the worker takes the worker lock again after a restart of horus-db ([#224](https://github.com/hieplam/horus/issues/224)) ([#229](https://github.com/hieplam/horus/issues/229)) ([e29746c](https://github.com/hieplam/horus/commit/e29746c1e0e84a66d0a4cf027d43714d14eb168e))

## [0.1.1](https://github.com/hieplam/horus/compare/v0.1.0...v0.1.1) (2026-11-18)


### Bug Fixes

* **doctor:** check comfy-mcp and .mcp.json only in a clone ([#174](https://github.com/hieplam/horus/issues/174)) ([a43763a](https://github.com/hieplam/horus/commit/a43763a3beda359d6500349e7fa3d3d984803e6f))

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

Flutter 全平台打包命令速查手册

本文档整理了 Flutter 项目在 Android、iOS、Windows 和 Web 端的标准打包及深度优化（瘦身、混淆）命令。

🛠️ 核心通用准备

在执行任何 Release 打包命令前，建议先执行清理操作，防止缓存导致的问题：

flutter clean
flutter pub get


1. 🤖 Android 打包

Android 分为 APK（直接安装）和 App Bundle（上架 Google Play）。

1.1 标准 Release 打包

生成通用的 APK（包含所有 CPU 架构，体积较大）：

flutter build apk --release


1.2 ✅ 极致精简打包 (推荐)

通过 --split-per-abi 将不同 CPU 架构（如 arm64-v8a, armeabi-v7a）拆分成独立的 APK，用户下载时体积显著减小。

flutter build apk --release --split-per-abi

只针对安卓arm设备
flutter build apk --release --target-platform android-arm64


输出位置: build/app/outputs/flutter-apk/ (会生成多个 apk 文件)

1.3 Google Play 上架打包 (AAB)

Google Play 目前强制要求使用 AAB 格式。

flutter build appbundle --release


1.4 代码混淆与符号表剥离 (安全性 + 进一步瘦身)

如果不希望代码被反编译，且需要进一步减小体积，必须加上混淆参数。
注意：请保存好生成的符号表文件，否则后续无法解析 Crash 日志。

flutter build apk --release --obfuscate --split-debug-info=./debug-info


--obfuscate: 开启混淆。

--split-debug-info=./debug-info: 将调试符号剥离并输出到当前目录的 debug-info 文件夹中。

2. 🍎 iOS 打包

iOS 打包通常生成 .ipa 文件用于上传 App Store 或 TestFlight。

2.1 构建归档 (Archive) 并生成 IPA

这是最标准的发布命令，构建完成后会生成 build/ios/archive/Runner.xcarchive。

flutter build ipa --release


如果配置了 export options，它也可以直接导出 IPA。通常情况下，执行完此命令后，开发者会使用 Xcode 打开 ios/Runner.xcworkspace 进行上传。

2.2 无签名构建 (仅用于测试/CI)

如果你在没有证书的机器上跑 CI/CD 流程：

flutter build ios --release --no-codesign


2.3 iOS 混淆与瘦身

与 Android 类似，iOS 也可以剥离符号表来减小体积。

flutter build ipa --release --obfuscate --split-debug-info=./debug-info


3. 🌐 Web 打包

Web 打包的核心在于选择渲染器 (Renderer)，这直接决定了网页的加载速度和兼容性。

3.1 自动选择 (默认)

移动端使用 HTML 渲染，桌面端使用 CanvasKit。

flutter build web --release


3.2 ✅ HTML 渲染 (体积最小，兼容性最好)

如果你的应用以文字、简单布局为主，追求首屏加载速度，使用此模式。体积比 CanvasKit 小 2MB+。

flutter build web --release --web-renderer html


3.3 CanvasKit 渲染 (性能最强，像素完美)

如果应用包含复杂的图形、动画或需要保证字体渲染完全一致，使用此模式。缺点是需要下载 canvaskit.wasm，首次加载较慢。

flutter build web --release --web-renderer canvaskit


3.4 PWA 优化 (去掉 PWA 缓存)

如果你不希望浏览器缓存太重（例如频繁更新版本），可以调整 PWA 策略：

flutter build web --release --pwa-strategy=none


4. 🪟 Windows 打包

Windows 打包会生成 .exe 可执行文件及其依赖库。

4.1 标准 Release 打包

flutter build windows --release


输出位置: build/windows/runner/Release/

4.2 压缩与分发

Flutter Windows 构建生成的是一个文件夹，包含 exe 和 dll 文件。
精简技巧：

构建完成后，进入输出目录。

即使是 Release 模式，也可以手动检查是否有冗余的资源文件。

必须将整个文件夹打包发送给用户，不能只发 .exe。

建议使用工具（如 Inno Setup）将该文件夹制作成一个安装包（Setup.exe），可以显著压缩体积。

5. 📉 终极体积优化总结 (Checklist)

如果你发现打包出来的 APP 体积依然很大，请检查以下几点：

图片资源：

是否包含过大的本地图片？建议使用 WebP 格式或压缩图片。

是否引入了未使用的 assets？

字体文件：

中文字体通常很大（10MB+），建议使用精简版字体或通过 Google Fonts 按需加载。

无用包清理：

检查 pubspec.yaml，移除未使用的第三方库。

架构拆分：

Android 务必使用 --split-per-abi。

6. 常用组合命令示例

Android 生产环境最佳实践 (混淆 + 拆分架构):

flutter clean
flutter build apk --release --obfuscate --split-debug-info=./symbols --split-per-abi


iOS 生产环境最佳实践:

flutter clean
flutter build ipa --release --obfuscate --split-debug-info=./symbols


Web 追求加载速度最佳实践:

flutter clean
flutter build web --release --web-renderer html

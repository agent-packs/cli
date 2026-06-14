---
name: mobile-development
description: Build production-quality mobile applications for iOS and Android using React Native or Flutter. Use when writing mobile UI, handling platform permissions, managing offline state, or optimizing app performance.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Mobile Development

Mobile apps run on devices you don't control. Design for offline, memory constraints, and slow networks from the start.

## UI and Navigation

- Use a declarative navigation library (React Navigation, Go Router) rather than imperative stack manipulation. Navigation state should be serializable for deep linking.
- Support deep links and universal links from day one — retrofitting them is painful. Map every screen to a URL.
- Design for safe areas: iOS notch, Android status bar, and home indicator eat into usable screen space. Use `SafeAreaView` / `SafeAreaProvider` or equivalent.
- Support both orientations unless the app's UX is genuinely portrait-only. Locking to portrait breaks tablets and landscape-preferred users.
- Keep gestures consistent with platform conventions: back gesture on Android, swipe-to-go-back on iOS. Don't fight the platform.

## Performance

- Avoid rendering inside `onScroll` handlers — debounce or use virtualized list components (`FlatList`, `LazyColumn`) that recycle cells.
- Move heavy computation (image processing, JSON parsing of large payloads) off the JS/Dart thread using native modules or isolates.
- Profile with platform tools: Xcode Instruments (iOS) and Android Studio Profiler. Wall-clock time in a dev build is not representative of release.
- Target 60fps (16ms per frame) for scroll and animation. 120fps on ProMotion displays requires explicit opt-in.
- Defer non-critical work using `InteractionManager.runAfterInteractions` (RN) or `WidgetsBinding.instance.addPostFrameCallback` (Flutter).

## Network and Offline

- Assume the network is unavailable at any point. Cache critical data locally (SQLite, Hive, MMKV) and sync opportunistically.
- Implement optimistic UI updates for user actions: show the result immediately, roll back on error.
- Use exponential backoff with jitter for network retries. Never retry in a tight loop.
- Show meaningful offline states — empty screens with "no connection" are better than spinner-forever states.
- Handle background app refresh carefully: iOS background fetch is time-limited and may be denied. Design sync to resume from where it stopped.

## Platform Permissions

- Request permissions just-in-time, immediately before the first use. Never request permissions on app launch.
- Handle all three states: granted, denied, and denied-permanently. Show rationale UI before the OS prompt.
- For sensitive permissions (location, microphone, camera), explain the specific use case in the rationale. "App needs location" is not a rationale.
- Test permission flows in a clean install state — cached permission state in development hides cold-start bugs.

## App Size and Build

- Analyze the bundle with `react-native-bundle-visualizer` (RN) or `--analyze` (Flutter). Find and remove unused dependencies.
- Enable Hermes (RN) and AOT compilation (Flutter release mode) — they significantly reduce startup time.
- Split assets into multiple APK/AAB splits by ABI or use `--split-per-abi` to avoid shipping arm64 bytecode to armv7 devices.
- Test release builds before submission — dev builds have different performance characteristics, JS source maps, and debug settings.

## Checklist

- [ ] Deep linking configured for all main screens.
- [ ] Safe area insets handled on all platforms.
- [ ] Virtualized lists used for any list > 50 items.
- [ ] Offline state handled gracefully with local cache.
- [ ] Permissions requested just-in-time with rationale UI.
- [ ] Release build tested on a physical device before submission.
- [ ] App bundle analyzed for unused dependencies.

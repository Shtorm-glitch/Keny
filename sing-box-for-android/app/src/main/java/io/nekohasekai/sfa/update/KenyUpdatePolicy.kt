package io.nekohasekai.sfa.update

object KenyUpdatePolicy {
    fun selectApk(names: List<String>, supportedAbis: List<String>, legacy: Boolean): String? {
        val candidates = names.filter {
            it.startsWith("keny.android-") && it.endsWith(".apk") &&
                it.contains("legacy-android-5") == legacy
        }
        for (abi in supportedAbis) {
            candidates.firstOrNull { it.endsWith("-$abi.apk") }?.let { return it }
        }
        return candidates.firstOrNull { it.endsWith("-universal.apk") }
    }
}

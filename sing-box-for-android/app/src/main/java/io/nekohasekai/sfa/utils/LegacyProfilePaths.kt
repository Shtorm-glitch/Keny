package io.nekohasekai.sfa.utils

internal fun remapLegacyProfilePath(path: String, internalFiles: String, externalFiles: String?): String {
    val roots = linkedMapOf(
        "/data/user/0/io.nekohasekai.sfa/files/" to "$internalFiles/",
        "/data/data/io.nekohasekai.sfa/files/" to "$internalFiles/",
    )
    if (externalFiles != null) {
        roots["/storage/emulated/0/Android/data/io.nekohasekai.sfa/files/"] = "$externalFiles/"
        roots["/sdcard/Android/data/io.nekohasekai.sfa/files/"] = "$externalFiles/"
    }
    for ((oldRoot, newRoot) in roots) {
        if (path.startsWith(oldRoot)) return newRoot + path.removePrefix(oldRoot)
    }
    return path
}

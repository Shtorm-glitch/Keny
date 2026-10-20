package io.nekohasekai.sfa.utils

import android.os.RemoteException

// A system hook may still belong to a different installed package/UID until
// its next activation. Denied or disconnected optional IPC must not stop the app.
internal inline fun <T> optionalHookQuery(block: () -> T): T? = try {
    block()
} catch (_: SecurityException) {
    null
} catch (_: RemoteException) {
    null
}

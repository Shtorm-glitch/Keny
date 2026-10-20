package io.nekohasekai.sfa.utils

import android.content.Context
import android.os.IBinder
import android.os.Parcel
import android.util.Log

object ConnectivityBinderUtils {
    private const val TAG = "ConnectivityBinderUtils"

    fun getBinder(): IBinder? {
        // ConnectivityManager.mService is blocked at target API 35. Keep the
        // existing service-manager bridge; report unavailable if access is denied.
        return try {
            val serviceManager = Class.forName("android.os.ServiceManager")
            val getService = serviceManager.getMethod("getService", String::class.java)
            getService.invoke(null, Context.CONNECTIVITY_SERVICE) as? IBinder
        } catch (e: Throwable) {
            Log.w(TAG, "Failed to get binder from ServiceManager", e)
            null
        }
    }

    inline fun <T> withParcel(block: (data: Parcel, reply: Parcel) -> T): T {
        val data = Parcel.obtain()
        val reply = Parcel.obtain()
        return try {
            block(data, reply)
        } finally {
            reply.recycle()
            data.recycle()
        }
    }
}

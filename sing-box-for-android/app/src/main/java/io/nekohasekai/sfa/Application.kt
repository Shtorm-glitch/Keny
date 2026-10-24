package io.nekohasekai.sfa

import android.app.Application
import android.app.NotificationManager
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.ConnectivityManager
import android.net.wifi.WifiManager
import android.os.PowerManager
import androidx.core.content.getSystemService
import go.Seq
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.libbox.SetupOptions
import io.nekohasekai.sfa.bg.AppChangeReceiver
import io.nekohasekai.sfa.bg.UpdateProfileWork
import io.nekohasekai.sfa.constant.Bugs
import io.nekohasekai.sfa.database.ProfileManager
import io.nekohasekai.sfa.utils.remapLegacyProfilePath
import io.nekohasekai.sfa.utils.AppLifecycleObserver
import io.nekohasekai.sfa.utils.HookModuleUpdateNotifier
import io.nekohasekai.sfa.utils.HookStatusClient
import io.nekohasekai.sfa.utils.PrivilegeSettingsClient
import io.nekohasekai.sfa.vendor.Vendor
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import java.io.File
import java.util.Locale
import io.nekohasekai.sfa.Application as BoxApplication

class Application : Application() {
    override fun attachBaseContext(base: Context?) {
        super.attachBaseContext(base)
        application = this
    }

    override fun onCreate() {
        super.onCreate()
        AppLifecycleObserver.register(this)

//        Seq.setContext(this)
        Libbox.setLocale(Locale.getDefault().toLanguageTag().replace("-", "_"))
        HookStatusClient.register(this)
        PrivilegeSettingsClient.register(this)

        @Suppress("OPT_IN_USAGE")
        GlobalScope.launch(Dispatchers.IO) {
            initialize()
            UpdateProfileWork.reconfigureUpdater()
            HookModuleUpdateNotifier.sync(this@Application)
            AppLifecycleObserver.isForeground.collect { foreground ->
                if (foreground) UpdateProfileWork.updateOnOpen()
            }
        }

        if (Vendor.isPerAppProxyAvailable()) {
            registerReceiver(
                AppChangeReceiver(),
                IntentFilter().apply {
                    addAction(Intent.ACTION_PACKAGE_ADDED)
                    addAction(Intent.ACTION_PACKAGE_REPLACED)
                    addDataScheme("package")
                },
            )
        }
    }

    private suspend fun initialize() {
        val baseDir = filesDir
        baseDir.mkdirs()
        val workingDir = getExternalFilesDir(null) ?: return
        workingDir.mkdirs()
        if (packageName == "keny.sfa") {
            val migratedProfiles = ProfileManager.list().filter { profile ->
                val previous = profile.typed.path
                val replacement = remapLegacyProfilePath(previous, baseDir.path, workingDir.path)
                if (replacement != previous && File(replacement).isFile) {
                    profile.typed.path = replacement
                    true
                } else {
                    false
                }
            }
            if (migratedProfiles.isNotEmpty()) ProfileManager.update(migratedProfiles)
        }
        val tempDir = cacheDir
        tempDir.mkdirs()
        Libbox.setup(
            SetupOptions().also {
                it.basePath = baseDir.path
                it.workingPath = workingDir.path
                it.tempPath = tempDir.path
                it.fixAndroidStack = Bugs.fixAndroidStack
                it.logMaxLines = 3000
                it.debug = BuildConfig.DEBUG
				it.crashReportSource = "Keny"
				it.appVersion = BuildConfig.VERSION_NAME
            },
        )
    }

    companion object {
        lateinit var application: BoxApplication
        val notification by lazy { application.getSystemService<NotificationManager>()!! }
        val connectivity by lazy { application.getSystemService<ConnectivityManager>()!! }
        val packageManager by lazy { application.packageManager }
        val powerManager by lazy { application.getSystemService<PowerManager>()!! }
        val notificationManager by lazy { application.getSystemService<NotificationManager>()!! }
        val wifiManager by lazy { application.getSystemService<WifiManager>()!! }
        val clipboard by lazy { application.getSystemService<ClipboardManager>()!! }
    }
}

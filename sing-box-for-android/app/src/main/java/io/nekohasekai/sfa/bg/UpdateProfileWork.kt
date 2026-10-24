package io.nekohasekai.sfa.bg

import android.content.Context
import android.util.Log
import androidx.work.BackoffPolicy
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequest
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.sfa.Application
import io.nekohasekai.sfa.database.ProfileManager
import io.nekohasekai.sfa.database.Settings
import io.nekohasekai.sfa.database.TypedProfile
import io.nekohasekai.sfa.utils.HTTPClient
import android.util.AtomicFile
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.io.File
import java.util.Date
import java.util.concurrent.TimeUnit

class UpdateProfileWork {
    companion object {
        private const val WORK_NAME = "UpdateProfile"
        private const val TAG = "UpdateProfileWork"
        private val updateMutex = Mutex()

        suspend fun updateOnOpen() {
            val ids = Settings.simpleRemoteProfiles
            if (ids.isNotEmpty()) updateProfiles(ids, onlyForced = true)
        }

        private suspend fun updateProfiles(forced: Set<String> = emptySet(), onlyForced: Boolean = false): Boolean =
            withContext(Dispatchers.IO) {
                updateMutex.withLock {
                    var success = true
                    var updatedSelected: Long? = null
                    for (profile in ProfileManager.list()) {
                        if (profile.typed.type != TypedProfile.Type.Remote || !profile.typed.autoUpdate) continue
                        val force = profile.id.toString() in forced
                        if (onlyForced && !force) continue
                        val age = System.currentTimeMillis() - profile.typed.lastUpdated.time
                        if (!force && age < profile.typed.autoUpdateInterval * 60_000L) continue
                        try {
                            val content = HTTPClient().use { it.getString(profile.typed.remoteURL) }
                            Libbox.checkConfig(content)
                            val fresh = ProfileManager.get(profile.id) ?: continue
                            // Discard a result whose URL was edited in Pro while downloading.
                            if (fresh.typed.remoteURL != profile.typed.remoteURL) continue
                            val file = File(fresh.typed.path)
                            if (!file.exists() || file.readText() != content) {
                                val atomic = AtomicFile(file)
                                val stream = atomic.startWrite()
                                try {
                                    stream.write(content.toByteArray(Charsets.UTF_8))
                                    atomic.finishWrite(stream)
                                } catch (e: Exception) {
                                    atomic.failWrite(stream)
                                    throw e
                                }
                                if (fresh.id == Settings.selectedProfile) updatedSelected = fresh.id
                            }
                            fresh.typed.lastUpdated = Date()
                            ProfileManager.update(fresh)
                        } catch (e: CancellationException) {
                            throw e
                        } catch (_: Exception) {
                            // URLs, profile names and config errors can contain private data.
                            Log.w(TAG, "Remote profile update failed; retaining saved config")
                            success = false
                        }
                    }
                    if (updatedSelected != null && updatedSelected == Settings.selectedProfile && Settings.startedByUser) {
                        runCatching { Libbox.newStandaloneCommandClient().serviceReload() }
                    }
                    success
                }
            }

        suspend fun reconfigureUpdater() {
            runCatching {
                reconfigureUpdater0()
            }.onFailure {
                Log.e(TAG, "reconfigureUpdater", it)
            }
        }

        private suspend fun reconfigureUpdater0() {
            val remoteProfiles =
                ProfileManager.list()
                    .filter { it.typed.type == TypedProfile.Type.Remote && it.typed.autoUpdate }
            if (remoteProfiles.isEmpty()) {
                WorkManager.getInstance(Application.application).cancelUniqueWork(WORK_NAME)
                return
            }

            var minDelay =
                remoteProfiles.minByOrNull { it.typed.autoUpdateInterval }!!.typed.autoUpdateInterval.toLong()
            val nowSeconds = System.currentTimeMillis() / 1000L
            val minInitDelay =
                remoteProfiles.minOf { (it.typed.autoUpdateInterval * 60) - (nowSeconds - (it.typed.lastUpdated.time / 1000L)) }
            if (minDelay < 15) minDelay = 15
            WorkManager.getInstance(Application.application).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.UPDATE,
                PeriodicWorkRequest.Builder(UpdateTask::class.java, minDelay, TimeUnit.MINUTES)
                    .apply {
                        if (minInitDelay > 0) setInitialDelay(minInitDelay, TimeUnit.SECONDS)
                        setBackoffCriteria(BackoffPolicy.LINEAR, 15, TimeUnit.MINUTES)
                    }
                    .build(),
            )
        }
    }

    class UpdateTask(appContext: Context, params: WorkerParameters) : CoroutineWorker(appContext, params) {
        override suspend fun doWork(): Result {
            return if (updateProfiles()) {
                Result.success()
            } else {
                Result.retry()
            }
        }
    }
}

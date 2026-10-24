package io.nekohasekai.sfa.compose.screen.simple

import android.app.Application
import android.net.Uri
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.sfa.bg.UpdateProfileWork
import io.nekohasekai.sfa.database.Profile
import io.nekohasekai.sfa.database.ProfileManager
import io.nekohasekai.sfa.database.Settings
import io.nekohasekai.sfa.database.TypedProfile
import io.nekohasekai.sfa.utils.HTTPClient
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.util.Date

class SimpleProfilesViewModel(application: Application) : AndroidViewModel(application) {
    private val _busy = MutableStateFlow(false)
    val busy = _busy.asStateFlow()
    private val _error = MutableStateFlow<String?>(null)
    val error = _error.asStateFlow()

    fun clearError() { _error.value = null }

    fun rename(profile: Profile, name: String, onDone: () -> Unit) {
        if (name.isBlank() || _busy.value) return
        perform("Не удалось переименовать профиль") {
            // Read a fresh record so an overlapping background update is preserved.
            val fresh = ProfileManager.get(profile.id) ?: error("Profile removed")
            fresh.name = name.trim()
            ProfileManager.update(fresh)
            withContext(Dispatchers.Main) { onDone() }
        }
    }

    fun add(name: String, url: String, uri: Uri?, onCreated: (Long) -> Unit) {
        if (_busy.value) return
        if (name.isBlank()) { _error.value = "Укажите название профиля"; return }
        if (uri == null && (Uri.parse(url).scheme !in listOf("http", "https") || Uri.parse(url).host.isNullOrBlank())) {
            _error.value = "Укажите ссылку http:// или https://"; return
        }
        perform("Не удалось загрузить профиль. Проверьте файл или ссылку и попробуйте снова.") {
            val context = getApplication<Application>()
            val content = if (uri != null) {
                context.contentResolver.openInputStream(uri)?.use { it.bufferedReader().readText() }
                    ?: error("Cannot open file")
            } else {
                HTTPClient().use { it.getString(url.trim()) }
            }
            Libbox.checkConfig(content)
            val directory = File(context.filesDir, "configs").also { it.mkdirs() }
            // A unique name avoids colliding with a concurrent import from Pro.
            val file = File.createTempFile("simple-", ".json", directory)
            try {
                file.writeText(content)
                val profile = Profile(name = name.trim(), typed = TypedProfile().apply {
                    path = file.path
                    type = if (uri == null) TypedProfile.Type.Remote else TypedProfile.Type.Local
                    if (uri == null) {
                        remoteURL = url.trim()
                        autoUpdate = true
                        autoUpdateInterval = 120
                        lastUpdated = Date()
                    }
                }).apply { userOrder = ProfileManager.nextOrder() }
                ProfileManager.create(profile)
                if (uri == null) {
                    Settings.simpleRemoteProfiles = Settings.simpleRemoteProfiles + profile.id.toString()
                    UpdateProfileWork.reconfigureUpdater()
                }
                withContext(Dispatchers.Main) { onCreated(profile.id) }
            } catch (e: Exception) {
                // Only remove an unreferenced file if the DB insert did not succeed.
                if (ProfileManager.list().none { it.typed.path == file.path }) file.delete()
                throw e
            }
        }
    }

    private fun perform(message: String, action: suspend () -> Unit) {
        _busy.value = true
        _error.value = null
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { action() }
            } catch (e: CancellationException) {
                throw e
            } catch (_: Exception) {
                _error.value = message
            } finally {
                _busy.value = false
            }
        }
    }
}

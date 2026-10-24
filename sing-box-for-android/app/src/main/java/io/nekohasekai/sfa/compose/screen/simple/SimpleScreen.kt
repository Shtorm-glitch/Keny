package io.nekohasekai.sfa.compose.screen.simple

import android.net.Uri
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import io.nekohasekai.sfa.BuildConfig
import io.nekohasekai.sfa.R
import io.nekohasekai.sfa.compose.screen.dashboard.DashboardViewModel
import io.nekohasekai.sfa.constant.Status
import io.nekohasekai.sfa.database.Profile
import io.nekohasekai.sfa.update.UpdateState
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val KenyGreen = Color(0xFF21C866)
private enum class ProfileSheet { List, Add, Link, File }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SimpleScreen(
    serviceStatus: Status,
    failureEvent: Long,
    recovering: Boolean,
    onToggle: () -> Unit,
    onOpenSettings: () -> Unit,
    dashboard: DashboardViewModel,
    profilesModel: SimpleProfilesViewModel = viewModel(),
) {
    val state by dashboard.uiState.collectAsState()
    val busy by profilesModel.busy.collectAsState()
    val error by profilesModel.error.collectAsState()
    val context = LocalContext.current
    var sheet by remember { mutableStateOf<ProfileSheet?>(null) }
    var actions by remember { mutableStateOf<Profile?>(null) }
    var rename by remember { mutableStateOf<Profile?>(null) }
    var delete by remember { mutableStateOf<Profile?>(null) }
    var name by rememberSaveable { mutableStateOf("") }
    var url by remember { mutableStateOf("") }
    var file by remember { mutableStateOf<Uri?>(null) }
    var failed by remember { mutableStateOf(false) }
    val pulse = remember { Animatable(0f) }
    LaunchedEffect(failureEvent) {
        val remaining = if (failureEvent == 0L) 0L else
            (5_000L - (android.os.SystemClock.elapsedRealtime() - failureEvent)).coerceIn(0L, 5_000L)
        if (remaining > 0L) {
            failed = true
            delay(remaining)
        }
        failed = false
    }
    LaunchedEffect(serviceStatus) {
        if (serviceStatus == Status.Starting) failed = false
    }
    LaunchedEffect(failed) {
        if (failed) {
            // Two short beats followed by a rest, for a five-second indication.
            while (true) {
                pulse.animateTo(1f, tween(110))
                pulse.animateTo(0.2f, tween(110))
                pulse.animateTo(0.85f, tween(110))
                pulse.animateTo(0f, tween(220))
                delay(450)
            }
        } else pulse.snapTo(0f)
    }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) {
            file = uri
            name = runCatching {
                context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use {
                    if (it.moveToFirst()) it.getString(0).substringBeforeLast('.') else ""
                }
            }.getOrNull().orEmpty()
            sheet = ProfileSheet.File
        }
    }
    val dark = isSystemInDarkTheme()
    val active = serviceStatus == Status.Started && !recovering && !failed
    val connecting = serviceStatus == Status.Starting || recovering
    val statusText = when {
        failed -> "Не удалось"
        serviceStatus == Status.Stopping -> "Отключение…"
        connecting -> "Попытка подкл."
        active -> "Подключено"
        else -> "Отключено"
    }
    val ink = if (dark) Color(0xFFF5F5F5) else Color(0xFF25282B)
    val face = when {
        active && dark -> Color(0xFF073E24)
        active -> Color(0xFFBEF0D0)
        dark -> Color(0xFF292C30)
        else -> Color(0xFFE3E4E6)
    }
    Scaffold(
        topBar = {
            CenterAlignedTopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text("Keny", fontWeight = FontWeight.Bold)
                        Icon(painterResource(R.drawable.keny_wolf_outline), null, Modifier.size(30.dp), tint = ink)
                    }
                },
                navigationIcon = {
                    IconButton(onClick = onOpenSettings) { Icon(Icons.Default.Settings, "Настройки") }
                },
            )
        },
    ) { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState())
                .padding(horizontal = 28.dp, vertical = 24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            Spacer(Modifier.height(12.dp))
            Surface(shape = RoundedCornerShape(22.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
                Column(Modifier.padding(horizontal = 26.dp, vertical = 12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("Состояние", style = MaterialTheme.typography.labelMedium)
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (connecting && !failed) {
                            CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp)
                        } else {
                            Box(Modifier.size(10.dp).background(when {
                                failed -> Color(0xFFE54949)
                                active -> KenyGreen
                                else -> Color.Gray
                            }, CircleShape))
                        }
                        Text(statusText, style = MaterialTheme.typography.titleMedium)
                    }
                }
            }
            Box(
                Modifier.size(232.dp).drawBehind {
                    val ring = when { failed -> Color(0xFFEF4444); active -> KenyGreen; else -> Color.Transparent }
                    val intensity = if (failed) pulse.value else 0.65f
                    for (i in 10 downTo 1) {
                        drawCircle(ring.copy(alpha = intensity * 0.028f),
                            radius = 100.dp.toPx() + i.dp.toPx(), style = Stroke((2 + i).dp.toPx()))
                    }
                },
                contentAlignment = Alignment.Center,
            ) {
                Surface(
                    modifier = Modifier.size(200.dp).semantics { contentDescription = if (active || connecting) "Отключить VPN" else "Подключить VPN" },
                    shape = CircleShape,
                    color = face,
                    border = BorderStroke(2.dp, if (failed) Color(0xFFEF4444) else if (active) KenyGreen else MaterialTheme.colorScheme.outlineVariant),
                    onClick = {
                        if (state.selectedProfileName == null && serviceStatus == Status.Stopped) sheet = ProfileSheet.Add
                        else { failed = false; onToggle() }
                    },
                    enabled = serviceStatus != Status.Stopping,
                ) {
                    Column(Modifier.fillMaxSize().padding(18.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                        Icon(Icons.Default.PowerSettingsNew, null, Modifier.size(44.dp), tint = ink)
                        Spacer(Modifier.height(4.dp))
                        Icon(painterResource(R.drawable.keny_wolf_outline), null, Modifier.width(126.dp).height(100.dp), tint = ink)
                    }
                }
                if (connecting && !failed) CircularProgressIndicator(Modifier.size(212.dp), color = KenyGreen, strokeWidth = 2.dp)
            }
            TextButton(onClick = { sheet = if (state.profiles.isEmpty()) ProfileSheet.Add else ProfileSheet.List }) {
                Text(state.selectedProfileName ?: "Добавьте профиль", maxLines = 2, overflow = TextOverflow.Ellipsis)
                Icon(Icons.Default.ExpandMore, null)
            }
            FilledTonalButton(
                onClick = { sheet = if (state.profiles.isEmpty()) ProfileSheet.Add else ProfileSheet.List },
                modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp).widthIn(max = 420.dp),
            ) { Icon(Icons.Default.List, null); Spacer(Modifier.width(10.dp)); Text("Профили") }
        }
    }

    if (sheet != null) {
        ModalBottomSheet(
            onDismissRequest = { if (!busy) { sheet = null; profilesModel.clearError() } },
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        ) {
            Column(Modifier.fillMaxWidth().imePadding().verticalScroll(rememberScrollState()).padding(start = 24.dp, end = 24.dp, bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(when (sheet) { ProfileSheet.List -> "Профили"; ProfileSheet.Link -> "По ссылке"; ProfileSheet.File -> "Из файла"; else -> "Добавить профиль" }, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
                when (sheet) {
                    ProfileSheet.List -> {
                        state.profiles.forEach { profile ->
                            Row(Modifier.fillMaxWidth().clickable(enabled = !state.isLoading && !busy, role = Role.RadioButton) {
                                dashboard.selectProfile(profile.id); sheet = null
                            }, verticalAlignment = Alignment.CenterVertically) {
                                RadioButton(profile.id == state.selectedProfileId, onClick = null)
                                Spacer(Modifier.width(12.dp))
                                Text(profile.name, Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                                IconButton(onClick = { actions = profile }) { Icon(Icons.Default.MoreVert, "Действия с профилем ${profile.name}") }
                            }
                        }
                        FilledTonalButton(onClick = { profilesModel.clearError(); sheet = ProfileSheet.Add }, Modifier.fillMaxWidth()) {
                            Icon(Icons.Default.Add, null); Text("Добавить профиль")
                        }
                    }
                    ProfileSheet.Add -> {
                        OutlinedButton(onClick = { profilesModel.clearError(); picker.launch(arrayOf("*/*")) }, Modifier.fillMaxWidth()) {
                            Icon(Icons.Default.FolderOpen, null); Spacer(Modifier.width(12.dp)); Text("Из файла")
                        }
                        OutlinedButton(onClick = { profilesModel.clearError(); name = ""; url = ""; file = null; sheet = ProfileSheet.Link }, Modifier.fillMaxWidth()) {
                            Icon(Icons.Default.Link, null); Spacer(Modifier.width(12.dp)); Text("По ссылке")
                        }
                    }
                    else -> {
                        OutlinedTextField(name, { name = it }, Modifier.fillMaxWidth(), label = { Text("Название") }, singleLine = true, enabled = !busy)
                        if (sheet == ProfileSheet.Link) {
                            OutlinedTextField(url, { url = it }, Modifier.fillMaxWidth(), label = { Text("Ссылка") }, placeholder = { Text("https://…") }, singleLine = true, enabled = !busy)
                            Text("Обновление каждые 120 минут и при открытии приложения", style = MaterialTheme.typography.bodySmall)
                        }
                        if (error != null) Text(error!!, color = MaterialTheme.colorScheme.error)
                        Button(onClick = {
                            profilesModel.add(name, url, if (sheet == ProfileSheet.File) file else null) { id ->
                                dashboard.selectProfile(id)
                                sheet = null
                            }
                        }, modifier = Modifier.fillMaxWidth(), enabled = !busy && name.isNotBlank() && (sheet == ProfileSheet.File || url.isNotBlank())) {
                            if (busy) { CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp); Spacer(Modifier.width(12.dp)) }
                            Text(if (busy) "Добавление…" else "Добавить")
                        }
                    }
                }
            }
        }
    }
    actions?.let { profile ->
        ModalBottomSheet(onDismissRequest = { actions = null }) {
            Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(profile.name, style = MaterialTheme.typography.titleLarge)
                TextButton(onClick = { name = profile.name; rename = profile; actions = null; profilesModel.clearError() }) { Icon(Icons.Default.Edit, null); Text("Переименовать") }
                TextButton(onClick = { delete = profile; actions = null }) { Icon(Icons.Default.Delete, null, tint = MaterialTheme.colorScheme.error); Text("Удалить", color = MaterialTheme.colorScheme.error) }
            }
        }
    }
    rename?.let { profile ->
        AlertDialog(
            onDismissRequest = { if (!busy) rename = null },
            title = { Text("Переименовать профиль") },
            text = { Column { OutlinedTextField(name, { name = it }, label = { Text("Название") }, enabled = !busy); error?.let { Text(it, color = MaterialTheme.colorScheme.error) } } },
            confirmButton = { TextButton(onClick = { profilesModel.rename(profile, name) { rename = null } }, enabled = !busy && name.isNotBlank()) { Text("Сохранить") } },
            dismissButton = { TextButton(onClick = { rename = null }, enabled = !busy) { Text("Отмена") } },
        )
    }
    delete?.let { profile ->
        AlertDialog(
            onDismissRequest = { delete = null },
            title = { Text("Удалить профиль?") },
            text = { Text(profile.name) },
            confirmButton = { TextButton(onClick = { dashboard.deleteProfile(profile); delete = null }) { Text("Удалить", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { delete = null }) { Text("Отмена") } },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun InterfaceSettingsSheet(simple: Boolean, onModeChange: (Boolean) -> Unit, onDownload: () -> Unit, onDismiss: () -> Unit) {
    val scope = rememberCoroutineScope()
    val result by UpdateState.checkResult
    val checking by UpdateState.isChecking
    val available by UpdateState.hasUpdate
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Text("Настройки", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                listOf("Простой", "Pro").forEachIndexed { index, title ->
                    SegmentedButton(selected = simple == (index == 0), onClick = { onModeChange(index == 0) }, shape = SegmentedButtonDefaults.itemShape(index, 2)) { Text(title) }
                }
            }
            Button(onClick = { if (available) onDownload() else scope.launch { UpdateState.checkNow() } }, modifier = Modifier.fillMaxWidth(), enabled = !checking) {
                if (checking) { CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp); Spacer(Modifier.width(10.dp)) }
                Text(when { checking -> "Проверяем…"; available -> "Загрузить обновление"; else -> "Проверить обновление" })
            }
            HorizontalDivider()
            Text("Keny · ${BuildConfig.VERSION_NAME}", style = MaterialTheme.typography.bodyMedium)
            Text(when {
                available -> "Требуется обновление"
                checking -> "Проверяем…"
                result == UpdateState.CheckResult.Current -> "Актуальная версия"
                result == UpdateState.CheckResult.Unavailable -> "Сервер обновлений недоступен"
                else -> "Версия не проверена"
            }, color = when {
                available -> Color(0xFFCD8900)
                result == UpdateState.CheckResult.Current -> if (isSystemInDarkTheme()) KenyGreen else Color(0xFF087A37)
                else -> MaterialTheme.colorScheme.onSurfaceVariant
            })
        }
    }
}

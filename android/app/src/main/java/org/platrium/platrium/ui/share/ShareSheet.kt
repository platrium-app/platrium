package org.platrium.platrium.ui.share

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Business
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Event
import androidx.compose.material.icons.outlined.Group
import androidx.compose.material.icons.outlined.Language
import androidx.compose.material.icons.outlined.Lock
import androidx.compose.material.icons.outlined.PersonRemove
import androidx.compose.material.icons.outlined.ArrowDropDown
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.InputChip
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import org.platrium.platrium.R
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.data.files.DriveItem
import org.platrium.platrium.data.sharing.AccessLevelOption
import org.platrium.platrium.data.sharing.GeneralAccess
import org.platrium.platrium.data.sharing.Grant
import org.platrium.platrium.data.sharing.RoleOption
import org.platrium.platrium.data.sharing.SharingRepository
import org.platrium.platrium.data.sharing.Subject
import org.platrium.platrium.ui.common.Avatar
import org.platrium.platrium.ui.common.appViewModel
import org.platrium.platrium.ui.common.endOfDayIso
import org.platrium.platrium.ui.common.formatDate

/** "VIEWER" -> "Viewer", for roles the server didn't label. */
private fun humanize(value: String): String =
    value.lowercase().split('_').joinToString(" ") { it.replaceFirstChar(Char::uppercase) }

private fun roleLabel(roles: List<RoleOption>, role: String): String = roles.firstOrNull { it.role == role }?.label ?: humanize(role)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShareSheet(session: AccountSession, item: DriveItem, onDismiss: () -> Unit) {
    val vm = appViewModel(key = "share-${session.account.id}-${session.account.tokenId}-${item.id}") {
        ShareViewModel(SharingRepository(session.apollo), item)
    }
    val state by vm.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.navigationBarsPadding()) {
            val title = if (item.parentId == null) R.string.share_manage_title else R.string.share_title
            Text(
                stringResource(title, item.name),
                style = MaterialTheme.typography.titleLarge,
                maxLines = 1,
                overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                modifier = Modifier.padding(horizontal = 24.dp, vertical = 8.dp),
            )

            when (state.phase) {
                SharePhase.Loading -> Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator()
                }
                SharePhase.Denied -> Text(
                    stringResource(R.string.share_denied),
                    modifier = Modifier.padding(24.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                SharePhase.Failed -> Column(Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(state.loadError.orEmpty(), color = MaterialTheme.colorScheme.error)
                    OutlinedButton(onClick = vm::load) { Text(stringResource(R.string.retry)) }
                }
                SharePhase.Ready -> ReadyContent(state, vm, item, Modifier.weight(1f, fill = false))
            }

            state.error?.let {
                Text(
                    it,
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(horizontal = 24.dp, vertical = 8.dp),
                )
            }

            Row(
                Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (state.phase == SharePhase.Ready) {
                    TextButton(onClick = {
                        val kind = if (item.isFolder) "folder" else "file"
                        val link = "${session.server.url}/$kind/${item.id}"
                        context.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText(item.name, link))
                    }) {
                        Icon(Icons.Outlined.ContentCopy, null, Modifier.size(18.dp))
                        Text(stringResource(R.string.share_copy_link), Modifier.padding(start = 8.dp))
                    }
                } else {
                    Box {}
                }
                Button(onClick = onDismiss) { Text(stringResource(R.string.done)) }
            }
        }
    }
}

@Composable
private fun ReadyContent(state: ShareUiState, vm: ShareViewModel, item: DriveItem, modifier: Modifier = Modifier) {
    val access = state.access ?: return
    LazyColumn(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(0.dp)) {
        // ---- Add people -------------------------------------------------
        item(key = "picker") {
            Column(Modifier.padding(horizontal = 24.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = state.query,
                    onValueChange = vm::onQueryChange,
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text(stringResource(R.string.share_add_people)) },
                    placeholder = { Text(stringResource(R.string.share_search_hint)) },
                    singleLine = true,
                    enabled = !state.busy,
                )
                if (state.recipients.isNotEmpty()) {
                    androidx.compose.foundation.layout.FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        state.recipients.forEach { r ->
                            InputChip(
                                selected = false,
                                onClick = { vm.removeRecipient(r) },
                                label = { Text(r.name) },
                                trailingIcon = { Icon(Icons.Outlined.Close, null, Modifier.size(18.dp)) },
                            )
                        }
                    }
                }
            }
        }
        items(state.results, key = { "result-${it.id}" }) { subject ->
            ListItem(
                modifier = Modifier.clickable { vm.addRecipient(subject) },
                leadingContent = { SubjectAvatar(subject.type, subject.name) },
                headlineContent = { Text(subject.name) },
                supportingContent = subject.email?.let { { Text(it) } },
            )
        }
        if (state.recipients.isNotEmpty()) {
            item(key = "new-access") {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Choice(
                        label = roleLabel(state.roles, state.activeRole),
                        options = state.roles.map { it.role to it.label },
                        enabled = !state.busy,
                        onSelect = vm::setRole,
                    )
                    ExpiryButton(state.expiresAt, enabled = !state.busy, onChange = vm::setExpiry)
                    Box(Modifier.weight(1f), contentAlignment = Alignment.CenterEnd) {
                        Button(onClick = vm::share, enabled = !state.busy && state.activeRole.isNotEmpty()) {
                            if (state.busy) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                            else Text(stringResource(R.string.share_action))
                        }
                    }
                }
            }
        }

        // ---- People with access --------------------------------------------
        item(key = "people-header") { SectionTitle(stringResource(R.string.share_people_with_access)) }
        item(key = "owner") {
            ListItem(
                leadingContent = { SubjectAvatar(access.owner.type, access.owner.name) },
                headlineContent = { Text(access.owner.name + if (access.owner.isYou) " (${stringResource(R.string.share_you)})" else "") },
                supportingContent = (if (access.owner.type == "TENANT") null else access.owner.email)?.let { { Text(it) } },
                trailingContent = { Text(stringResource(R.string.share_owner), color = MaterialTheme.colorScheme.onSurfaceVariant) },
            )
        }
        items(access.grants, key = { "grant-${it.id}" }) { grant ->
            GrantRow(grant, state, vm, editable = !grant.isYou)
        }
        items(access.inherited, key = { "inherited-${it.id}" }) { grant ->
            GrantRow(grant, state, vm, editable = false)
        }
        if (access.grants.isEmpty() && access.inherited.isEmpty()) {
            item(key = "only-owner") {
                Text(
                    stringResource(R.string.share_owner_only),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 24.dp),
                )
            }
        }

        // ---- General access ------------------------------------------------
        item(key = "general") { GeneralAccessSection(access.generalAccess, state, vm) }

        // ---- Inheritance -----------------------------------------------------
        if (item.parentId != null) {
            item(key = "inheritance") {
                val kind = if (item.isFolder) "folder" else "file"
                HorizontalDivider(Modifier.padding(vertical = 8.dp))
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    Text(
                        stringResource(if (access.inheritsPermissions) R.string.share_inherit_on else R.string.share_inherit_off, kind),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.weight(1f),
                    )
                    OutlinedButton(onClick = { vm.setInheritance(!access.inheritsPermissions) }, enabled = !state.busy) {
                        Text(stringResource(if (access.inheritsPermissions) R.string.share_restrict else R.string.share_inherit))
                    }
                }
            }
        }
    }
}

@Composable
private fun GrantRow(grant: Grant, state: ShareUiState, vm: ShareViewModel, editable: Boolean) {
    val notes = listOfNotNull(
        grant.inheritedFromName?.let { stringResource(R.string.share_inherited_from, it) },
        if (grant.noDownload) stringResource(R.string.share_cant_download) else null,
        grant.expiresAt?.let { stringResource(R.string.share_expires_on, formatDate(it)) },
    )
    val busy = state.busy && state.busyGrantId == grant.id
    ListItem(
        leadingContent = { SubjectAvatar(grant.subjectType, grant.subjectName) },
        headlineContent = { Text(grant.subjectName + if (grant.isYou) " (${stringResource(R.string.share_you)})" else "") },
        supportingContent = if (notes.isNotEmpty()) { { Text(notes.joinToString(" · ")) } } else null,
        trailingContent = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (busy) {
                    CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                } else if (editable) {
                    Choice(
                        label = roleLabel(state.roles, grant.role),
                        options = state.roles.map { it.role to it.label },
                        enabled = !state.busy,
                        onSelect = { vm.changeRole(grant, it) },
                    )
                    IconButton(onClick = { vm.remove(grant) }, enabled = !state.busy) {
                        Icon(Icons.Outlined.PersonRemove, contentDescription = stringResource(R.string.share_remove_access))
                    }
                } else {
                    Text(roleLabel(state.roles, grant.role), color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        },
    )
}

@Composable
private fun GeneralAccessSection(value: GeneralAccess, state: ShareUiState, vm: ShareViewModel) {
    // A level set earlier but no longer offered (a policy changed) still shows.
    val current = state.levels.firstOrNull { it.level == value.level } ?: AccessLevelOption(
        level = value.level, label = humanize(value.level), blurb = "",
        roles = listOfNotNull(value.role), supportsExpiry = value.expiresAt != null,
    )
    val options = if (state.levels.any { it.level == current.level }) state.levels else listOf(current) + state.levels
    val takesRole = current.roles.isNotEmpty()
    val role = value.role ?: current.roles.firstOrNull()
    val downloadOptional = state.roles.firstOrNull { it.role == role }?.downloadOptional == true

    SectionTitle(stringResource(R.string.share_general_access))
    Column(Modifier.padding(horizontal = 24.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Choice(
                label = current.label,
                options = options.map { it.level to it.label },
                leadingIcon = levelIcon(current.level),
                enabled = !state.busy,
                onSelect = { next ->
                    val def = options.first { it.level == next }
                    if (def.roles.isEmpty()) {
                        vm.setGeneralAccess(next, null, false, null)
                    } else {
                        val nextRole = if (role != null && role in def.roles) role else def.roles.first()
                        val keepsDownload = state.roles.firstOrNull { it.role == nextRole }?.downloadOptional == true
                        vm.setGeneralAccess(next, nextRole, keepsDownload && value.noDownload, if (def.supportsExpiry) value.expiresAt else null)
                    }
                },
            )
            if (takesRole) {
                if (current.roles.size > 1) {
                    Choice(
                        label = roleLabel(state.roles, role.orEmpty()),
                        options = current.roles.map { it to roleLabel(state.roles, it) },
                        enabled = !state.busy,
                        onSelect = { next ->
                            val keepsDownload = state.roles.firstOrNull { it.role == next }?.downloadOptional == true
                            vm.setGeneralAccess(current.level, next, keepsDownload && value.noDownload, value.expiresAt)
                        },
                    )
                } else {
                    Text(roleLabel(state.roles, current.roles.first()), color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
        if (current.blurb.isNotEmpty()) {
            Text(current.blurb, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        if (takesRole && downloadOptional) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
                Text(stringResource(R.string.share_allow_download))
                Switch(
                    checked = !value.noDownload,
                    onCheckedChange = { vm.setGeneralAccess(current.level, role, !it, value.expiresAt) },
                    enabled = !state.busy,
                )
            }
        }
        if (takesRole && current.supportsExpiry) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                ExpiryButton(
                    value.expiresAt,
                    enabled = !state.busy,
                    onChange = { vm.setGeneralAccess(current.level, role, value.noDownload, it) },
                )
            }
        }
    }
}

@Composable
private fun levelIcon(level: String) = when (level) {
    "TENANT" -> Icons.Outlined.Business
    "PUBLIC" -> Icons.Outlined.Language
    else -> Icons.Outlined.Lock
}

@Composable
private fun SubjectAvatar(type: String, name: String) {
    when (type) {
        "TENANT" -> Icon(Icons.Outlined.Business, null, Modifier.size(40.dp).padding(8.dp))
        "GROUP" -> Icon(Icons.Outlined.Group, null, Modifier.size(40.dp).padding(8.dp))
        "PUBLIC" -> Icon(Icons.Outlined.Language, null, Modifier.size(40.dp).padding(8.dp))
        else -> Avatar(name, size = 40.dp)
    }
}

@Composable
private fun SectionTitle(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.labelLarge,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(start = 24.dp, end = 24.dp, top = 16.dp, bottom = 4.dp),
    )
}

/** A chip that opens a menu of (value, label) options. */
@Composable
private fun Choice(
    label: String,
    options: List<Pair<String, String>>,
    enabled: Boolean,
    onSelect: (String) -> Unit,
    leadingIcon: androidx.compose.ui.graphics.vector.ImageVector? = null,
) {
    var open by remember { mutableStateOf(false) }
    Box {
        AssistChip(
            onClick = { open = true },
            enabled = enabled,
            label = { Text(label) },
            leadingIcon = leadingIcon?.let { { Icon(it, null, Modifier.size(18.dp)) } },
            trailingIcon = { Icon(Icons.Outlined.ArrowDropDown, null, Modifier.size(18.dp)) },
        )
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            options.forEach { (value, text) ->
                DropdownMenuItem(text = { Text(text) }, onClick = { open = false; onSelect(value) })
            }
        }
    }
}

/** Pick an expiry date, or clear it. Value and result are ISO instants. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ExpiryButton(expiresAt: String?, enabled: Boolean, onChange: (String?) -> Unit) {
    var showPicker by remember { mutableStateOf(false) }
    AssistChip(
        onClick = { showPicker = true },
        enabled = enabled,
        label = { Text(expiresAt?.let { formatDate(it) } ?: stringResource(R.string.share_expires)) },
        leadingIcon = { Icon(Icons.Outlined.Event, null, Modifier.size(18.dp)) },
        trailingIcon = if (expiresAt != null) {
            { Icon(Icons.Outlined.Close, null, Modifier.size(18.dp).clickable { onChange(null) }) }
        } else null,
    )
    if (showPicker) {
        val tomorrow = System.currentTimeMillis() + 86_400_000
        val pickerState = rememberDatePickerState(
            selectableDates = object : androidx.compose.material3.SelectableDates {
                override fun isSelectableDate(utcTimeMillis: Long) = utcTimeMillis >= tomorrow - 86_400_000
            },
        )
        DatePickerDialog(
            onDismissRequest = { showPicker = false },
            confirmButton = {
                TextButton(
                    enabled = pickerState.selectedDateMillis != null,
                    onClick = {
                        pickerState.selectedDateMillis?.let { onChange(endOfDayIso(it)) }
                        showPicker = false
                    },
                ) { Text(stringResource(R.string.done)) }
            },
            dismissButton = { TextButton(onClick = { showPicker = false }) { Text(stringResource(R.string.cancel)) } },
        ) { DatePicker(state = pickerState) }
    }
}

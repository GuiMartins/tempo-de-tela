package com.tempodetela.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight

// Mesma paleta do painel web (layout.html), pra app e painel parecerem um produto só.
private val Light = lightColorScheme(
    primary = Color(0xFF3B6FD8), onPrimary = Color.White,
    primaryContainer = Color(0xFFE9F0FF), onPrimaryContainer = Color(0xFF1C3F8F),
    secondary = Color(0xFF55607A), onSecondary = Color.White,
    secondaryContainer = Color(0xFFE6EAF3), onSecondaryContainer = Color(0xFF283044),
    background = Color(0xFFF4F6FB), onBackground = Color(0xFF141824),
    surface = Color(0xFFF4F6FB), onSurface = Color(0xFF141824),
    surfaceVariant = Color(0xFFF1F3F8), onSurfaceVariant = Color(0xFF636B7E),
    surfaceContainerLowest = Color.White, surfaceContainerLow = Color.White,
    surfaceContainer = Color(0xFFF7F8FC), surfaceContainerHigh = Color(0xFFF1F3F8),
    surfaceContainerHighest = Color(0xFFE9ECF3),
    outline = Color(0xFFCFD5E2), outlineVariant = Color(0xFFE2E6EF),
    error = Color(0xFFCF3535), errorContainer = Color(0xFFFDE9E9), onErrorContainer = Color(0xFF8C1D1D),
)

private val Dark = darkColorScheme(
    primary = Color(0xFF7EA4FF), onPrimary = Color(0xFF0B1220),
    primaryContainer = Color(0xFF1B2744), onPrimaryContainer = Color(0xFFC9D8FF),
    secondary = Color(0xFFBEC6DC), onSecondary = Color(0xFF1B2130),
    secondaryContainer = Color(0xFF252C3B), onSecondaryContainer = Color(0xFFDCE2F0),
    background = Color(0xFF0D1016), onBackground = Color(0xFFE8EBF2),
    surface = Color(0xFF0D1016), onSurface = Color(0xFFE8EBF2),
    surfaceVariant = Color(0xFF1C212C), onSurfaceVariant = Color(0xFF9AA3B5),
    surfaceContainerLowest = Color(0xFF151922), surfaceContainerLow = Color(0xFF151922),
    surfaceContainer = Color(0xFF171C25), surfaceContainerHigh = Color(0xFF1C212C),
    surfaceContainerHighest = Color(0xFF242A36),
    outline = Color(0xFF343C4D), outlineVariant = Color(0xFF252B38),
    error = Color(0xFFFF7A7A), errorContainer = Color(0xFF341616), onErrorContainer = Color(0xFFFFC9C9),
)

/** Cores de estado que o Material3 não tem (online, aviso). */
@Immutable
data class StatusColors(val ok: Color, val okContainer: Color, val warn: Color, val warnContainer: Color)

private val LightStatus = StatusColors(Color(0xFF18915A), Color(0xFFE2F5EB), Color(0xFFB26A00), Color(0xFFFDF1DC))
private val DarkStatus = StatusColors(Color(0xFF41CF85), Color(0xFF12291D), Color(0xFFF2B443), Color(0xFF2E2410))

val LocalStatus = staticCompositionLocalOf { LightStatus }

private val base = Typography()
private val AppTypography = base.copy(
    headlineMedium = base.headlineMedium.copy(fontWeight = FontWeight.Bold),
    headlineSmall = base.headlineSmall.copy(fontWeight = FontWeight.Bold),
    titleLarge = base.titleLarge.copy(fontWeight = FontWeight.SemiBold),
    titleMedium = base.titleMedium.copy(fontWeight = FontWeight.SemiBold),
    titleSmall = base.titleSmall.copy(fontWeight = FontWeight.SemiBold),
)

@Composable
fun TempoTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    CompositionLocalProvider(LocalStatus provides if (dark) DarkStatus else LightStatus) {
        MaterialTheme(colorScheme = if (dark) Dark else Light, typography = AppTypography, content = content)
    }
}

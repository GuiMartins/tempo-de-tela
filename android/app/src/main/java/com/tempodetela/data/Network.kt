package com.tempodetela.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Response
import retrofit2.HttpException
import retrofit2.Retrofit
import java.io.IOException
import java.util.concurrent.TimeUnit

val apiJson = Json {
    ignoreUnknownKeys = true
    explicitNulls = true
}

/**
 * Tenta cada endereço do servidor na ordem (o último que funcionou primeiro): em casa responde o
 * IP da LAN, fora de casa o da Tailscale. Também põe o token em toda requisição.
 */
class FailoverInterceptor(
    private val config: () -> ServerConfig?,
    private val onWorking: (String) -> Unit,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val cfg = config() ?: throw IOException("Servidor não pareado")
        val original = chain.request()
        var last: IOException? = null
        for (base in cfg.orderedUrls()) {
            val baseUrl = base.toHttpUrlOrNull() ?: continue
            val url = original.url.newBuilder()
                .scheme(baseUrl.scheme).host(baseUrl.host).port(baseUrl.port).build()
            val request = original.newBuilder()
                .url(url)
                .header("Authorization", "Bearer ${cfg.token}")
                .build()
            try {
                val response = chain.proceed(request)
                if (base != cfg.preferred) onWorking(base)
                return response
            } catch (e: IOException) {
                last = e
            }
        }
        throw last ?: IOException("Nenhum endereço do servidor configurado")
    }
}

fun buildApi(config: () -> ServerConfig?, onWorking: (String) -> Unit = {}): Api {
    val client = OkHttpClient.Builder()
        // Conexão curta: o IP da LAN fora de casa não responde, e o próximo endereço é a Tailscale.
        .connectTimeout(3, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .addInterceptor(FailoverInterceptor(config, onWorking))
        .build()
    return Retrofit.Builder()
        .baseUrl("http://servidor.invalid/")
        .client(client)
        .addConverterFactory(apiJson.asConverterFactory("application/json".toMediaType()))
        .build()
        .create(Api::class.java)
}

/** Mensagem em português para mostrar ao usuário. */
fun Throwable.userMessage(): String = when (this) {
    is HttpException -> {
        val body = try {
            response()?.errorBody()?.string()?.let { apiJson.decodeFromString(ApiError.serializer(), it).error }
        } catch (e: Exception) {
            null
        }
        when {
            code() == 401 -> "O servidor recusou o acesso. Pareie o app de novo."
            !body.isNullOrBlank() -> body.replaceFirstChar { it.uppercase() } + "."
            else -> "O servidor respondeu com erro ${code()}."
        }
    }
    is IOException -> "Não consegui falar com o servidor. Confira se está em casa ou com a Tailscale ligada."
    is IllegalArgumentException -> message ?: "Dados inválidos."
    else -> message ?: "Algo deu errado."
}

val Throwable.isUnauthorized: Boolean get() = this is HttpException && code() == 401

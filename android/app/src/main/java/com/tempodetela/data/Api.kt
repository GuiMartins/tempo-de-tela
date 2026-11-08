package com.tempodetela.data

import kotlinx.serialization.json.JsonObject
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface Api {
    @GET("api/v1/status")
    suspend fun status(): ServerStatus

    @GET("api/v1/devices")
    suspend fun devices(@Query("filter") filter: String = "all"): List<Device>

    @PATCH("api/v1/devices/{id}")
    suspend fun patchDevice(@Path("id") id: Long, @Body body: JsonObject): Device

    /** Corpo {"person_id": N} ou {"person_id": null}. */
    @PUT("api/v1/devices/{id}/person")
    suspend fun assignDevice(@Path("id") id: Long, @Body body: JsonObject): Device

    @GET("api/v1/persons")
    suspend fun persons(): List<Person>

    @POST("api/v1/persons")
    suspend fun createPerson(@Body body: JsonObject): Person

    @PATCH("api/v1/persons/{id}")
    suspend fun patchPerson(@Path("id") id: Long, @Body body: JsonObject): Person

    @DELETE("api/v1/persons/{id}")
    suspend fun deletePerson(@Path("id") id: Long): Response<Unit>

    /** {"minutes": 60} pausa por 1 h; -1 sem prazo; 0 libera. */
    @POST("api/v1/persons/{id}/pause")
    suspend fun pause(@Path("id") id: Long, @Body body: JsonObject): Person

    @GET("api/v1/catalog")
    suspend fun catalog(): AppCatalog

    @GET("api/v1/catalog/unclassified")
    suspend fun unclassified(@Query("days") days: Int = 7): List<Unclassified>

    /** {"domain", "app_id"} ou {"domain", "new_app": {"name", "category"}}. */
    @POST("api/v1/catalog/rules")
    suspend fun createRule(@Body body: JsonObject): JsonObject

    @GET("api/v1/usage/today")
    suspend fun usageToday(): List<PersonToday>

    @GET("api/v1/persons/{id}/usage")
    suspend fun personUsage(@Path("id") id: Long, @Query("date") date: String, @Query("days") days: Int): UsageSummary

    @GET("api/v1/persons/{id}/rules")
    suspend fun rules(@Path("id") id: Long): PersonRules

    @PUT("api/v1/persons/{id}/rules")
    suspend fun saveRules(@Path("id") id: Long, @Body rules: PersonRules): PersonRules

    /** after = último id já visto (notificações); sem after, os mais recentes. */
    @GET("api/v1/events")
    suspend fun events(@Query("after") after: Long? = null, @Query("limit") limit: Int = 50): List<AlertEvent>

    @GET("api/v1/usage-alerts")
    suspend fun usageAlerts(): List<UsageAlert>

    /** {"app_id"} ou {"category"} (nenhum = qualquer uso) e {"device_id"} ou {"person_id"}. */
    @POST("api/v1/usage-alerts")
    suspend fun createUsageAlert(@Body body: JsonObject): UsageAlert

    @DELETE("api/v1/usage-alerts/{id}")
    suspend fun deleteUsageAlert(@Path("id") id: Long): Response<Unit>

    @GET("api/v1/devices/{id}/usage")
    suspend fun deviceUsage(@Path("id") id: Long, @Query("date") date: String, @Query("days") days: Int): UsageSummary
}

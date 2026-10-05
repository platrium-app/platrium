plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.apollo)
    alias(libs.plugins.ksp)
}

android {
    namespace = "org.platrium.platrium"
    compileSdk {
        version = release(36)
    }

    defaultConfig {
        applicationId = "org.platrium.platrium"
        minSdk = 33
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    testOptions {
        unitTests.isReturnDefaultValues = true
    }
}

ksp {
    arg("room.schemaLocation", "$projectDir/schemas")
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.service)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.androidx.browser)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material3.adaptive.navigation.suite)
    implementation(libs.androidx.compose.material.icons.extended)
    implementation(libs.kotlinx.coroutines.android)

    implementation(libs.androidx.room.runtime)
    implementation(libs.androidx.room.ktx)
    ksp(libs.androidx.room.compiler)

    // Platrium SDK (UniFFI Kotlin bindings) and Apollo GraphQL
    implementation(project(":platrium-sdk"))
    implementation(libs.apollo.runtime)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation(libs.androidx.compose.ui.test.junit4)
    debugImplementation(libs.androidx.compose.ui.tooling)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}

// The schema lives with the engine (api/graphql/core), shared with the web and
// Darwin clients. Apollo only picks up .graphqls files, so stage a copy.
val stageGraphqlSchema by tasks.registering(Sync::class) {
    from(rootProject.file("../api/graphql/core")) {
        include("*.graphql")
        rename { it.removeSuffix(".graphql") + ".graphqls" }
    }
    into(layout.buildDirectory.dir("graphql-schema"))
}

apollo {
    service("service") {
        packageName.set("org.platrium.platrium.graphql")
        generateKotlinModels.set(true)
        mapScalar("DateTime", "kotlin.String")
        mapScalar("Int64", "kotlin.Long")
        schemaFiles.from(fileTree(layout.buildDirectory.dir("graphql-schema")) {
            include("*.graphqls")
            builtBy(stageGraphqlSchema)
        })
    }
}

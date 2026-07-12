use platrium_sdk::client::PlatriumClient;

#[tokio::main]
async fn main() {
    println!("Uploading a File");
    let client = PlatriumClient::new("http://localhost:3000/api").unwrap();
    let res = client
        .files()
        .upload(
            "966dfad1-293b-48e8-8479-662032c8ec88",
            "logfile2.txt",
            "/tmp/test.txt",
        )
        .await
        .unwrap();

    print!("Upload Result: {}", res)
}

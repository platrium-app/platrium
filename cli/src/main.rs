use platrium_sdk::client::{PlatriumClient, files::UploadSource};

#[tokio::main]
async fn main() {
    println!("Uploading a File");
    let client = PlatriumClient::new("http://localhost:3000/api").unwrap();
    let upload_src = UploadSource::new(
        "gradescope_submission.pdf".to_string(),
        "/def/not/telling/u/whats/on/my/computer.pdf".to_string(),
    );

    let res = client
        .files()
        .upload("546920c2-da07-44bc-9d6d-4008bf772431", upload_src.into())
        .await
        .unwrap();

    print!("Upload Result: {}", res)
}

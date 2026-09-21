use platrium_sdk::client::{
    PlatriumClient,
    files::{DownloadDestination, UploadSource},
};

#[tokio::main]
async fn main() {
    println!("Uploading a File");
    let client = PlatriumClient::new("http://localhost:3000/api").unwrap();
    // let upload_src = UploadSource::new(
    //     "gradescope_submission.pdf".to_string(),
    //     "/def/not/telling/u/whats/on/my/computer.pdf".to_string(), // Breaking API Changes.
    // );

    // let res = client
    //     .files()
    //     .upload("546920c2-da07-44bc-9d6d-4008bf772431", upload_src.into())
    //     .await
    //     .unwrap();

    // print!("Upload Result: {}", res)

    let download_session = client
        .files()
        .create_download_session("db616064-7e3c-4433-8470-c8ed0d1bc428".to_string())
        .await
        .unwrap();

    let file_path = format!("/tmp/{}", download_session.file_name());
    let file = std::fs::File::create(&file_path).unwrap();
    let download_dest = DownloadDestination::new(file);

    download_session.stream_to(&download_dest).await.unwrap();
    println!("Saved File to {}", file_path);
}

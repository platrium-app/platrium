import PlatriumGraphQL
import SwiftUI
import Apollo

typealias FolderContentNode = PlatriumGraphQL.GetFolderContentsQuery.Data.FolderContents.Edge.Node

@Observable
final class FolderViewModel {
    private(set) var items: [FolderContentNode] = []
    private(set) var isLoading: Bool = false
    private(set) var isLoadingMore: Bool = false
    private(set) var errorMessage: String? = nil
    private(set) var hasNextPage: Bool = false
    private(set) var endCursor: String? = nil
    private(set) var folderName: String? = nil

    func loadInitial(folderId: String, apollo: ApolloClient) async {
        guard !isLoading else { return }
        
        self.items = []
        self.errorMessage = nil
        self.hasNextPage = false
        self.endCursor = nil
        self.folderName = nil
        self.isLoading = true
        
        async let infoFetch: () = fetchFolderInfo(folderId: folderId, apollo: apollo)
        async let contentFetch: () = fetch(folderId: folderId, after: nil, apollo: apollo)
        
        _ = await (infoFetch, contentFetch)
    }

    private func fetchFolderInfo(folderId: String, apollo: ApolloClient) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let query = PlatriumGraphQL.GetDriveItemInfoQuery(id: folderId)
            apollo.fetch(query: query, cachePolicy: .fetchIgnoringCacheData) { [weak self] result in
                DispatchQueue.main.async {
                    if case .success(let graphQLResult) = result {
                        self?.folderName = graphQLResult.data?.item?.name
                    }
                    continuation.resume()
                }
            }
        }
    }

    func loadMore(folderId: String, apollo: ApolloClient) async {
        guard !isLoading && !isLoadingMore && hasNextPage else { return }
        self.isLoadingMore = true
        await fetch(folderId: folderId, after: endCursor, apollo: apollo)
    }

    private func fetch(folderId: String, after: String?, apollo: ApolloClient) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let query = PlatriumGraphQL.GetFolderContentsQuery(
                folderId: folderId,
                first: .some(50),
                after: after != nil ? .some(after!) : .none
            )
            
            apollo.fetch(query: query, cachePolicy: .fetchIgnoringCacheData) { [weak self] result in
                guard let self else {
                    continuation.resume()
                    return
                }
                DispatchQueue.main.async {
                    self.isLoading = false
                    self.isLoadingMore = false
                    
                    switch result {
                    case .success(let graphQLResult):
                        if let connection = graphQLResult.data?.folderContents {
                            let newItems = connection.edges.map { $0.node }
                            
                            // Filter out duplicates if any (defensive approach)
                            let existingIds = Set(self.items.map { $0.id })
                            let uniqueNewItems = newItems.filter { !existingIds.contains($0.id) }
                            
                            self.items.append(contentsOf: uniqueNewItems)
                            self.hasNextPage = connection.pageInfo.hasNextPage
                            self.endCursor = connection.pageInfo.endCursor
                        } else if let firstError = graphQLResult.errors?.first {
                            self.errorMessage = firstError.message
                        }
                    case .failure(let error):
                        self.errorMessage = error.localizedDescription
                    }
                    continuation.resume()
                }
            }
        }
    }
}

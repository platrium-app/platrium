/* eslint-disable */
import * as types from './graphql';
import type { TypedDocumentNode as DocumentNode } from '@graphql-typed-document-node/core';

/**
 * Map of all GraphQL operations in the project.
 *
 * This map has several performance disadvantages:
 * 1. It is not tree-shakeable, so it will include all operations in the project.
 * 2. It is not minifiable, so the string of a GraphQL query will be multiple times inside the bundle.
 * 3. It does not support dead code elimination, so it will add unused operations.
 *
 * Therefore it is highly recommended to use the babel or swc plugin for production.
 * Learn more about it here: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client#reducing-bundle-size
 */
type Documents = {
    "\n  query GetAuthConfig {\n    tenantAuthConfig {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n": typeof types.GetAuthConfigDocument,
    "\n  mutation CreateFolder($parentId: ID!, $name: String!) {\n    createFolder(parentId: $parentId, name: $name) {\n      id\n      name\n    }\n  }\n": typeof types.CreateFolderDocument,
    "\n  mutation RenameItem($id: ID!, $newName: String!) {\n    renameItem(id: $id, newName: $newName) {\n      id\n      name\n    }\n  }\n": typeof types.RenameItemDocument,
    "\n  mutation MoveItem($id: ID!, $newParentId: ID!) {\n    moveItem(id: $id, newParentId: $newParentId) {\n      id\n      name\n      parentId\n    }\n  }\n": typeof types.MoveItemDocument,
    "\n  mutation CopyFile($fileId: ID!, $newParentId: ID!, $newName: String!) {\n    copyFile(fileId: $fileId, newParentId: $newParentId, newName: $newName) {\n      id\n      name\n      parentId\n    }\n  }\n": typeof types.CopyFileDocument,
    "\n  query GetDrivesForPicker {\n    drives {\n      id\n      name\n    }\n  }\n": typeof types.GetDrivesForPickerDocument,
    "\n  query GetSubfoldersPicker($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n": typeof types.GetSubfoldersPickerDocument,
    "\n  query ItemAccess($itemId: ID!) {\n    itemAccess(itemId: $itemId) {\n      itemId\n      inheritsPermissions\n      owner {\n        type\n        id\n        name\n        email\n        isYou\n      }\n      generalAccess {\n        level\n        role\n        noDownload\n        expiresAt\n      }\n      grants {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n      }\n      inherited {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n        inheritedFrom {\n          id\n          name\n        }\n      }\n    }\n  }\n": typeof types.ItemAccessDocument,
    "\n  query ShareRoles($itemId: ID!) {\n    shareRoles(itemId: $itemId) {\n      role\n      label\n      description\n      capabilities\n      downloadOptional\n    }\n  }\n": typeof types.ShareRolesDocument,
    "\n  query GeneralAccessOptions($itemId: ID!) {\n    generalAccessOptions(itemId: $itemId) {\n      level\n      label\n      blurb\n      roles\n      supportsExpiry\n    }\n  }\n": typeof types.GeneralAccessOptionsDocument,
    "\n  query SearchDirectory($query: String!, $first: Int, $exclude: ID) {\n    searchDirectory(\n      query: $query\n      first: $first\n      excludeAccessToItemId: $exclude\n    ) {\n      type\n      id\n      name\n      email\n    }\n  }\n": typeof types.SearchDirectoryDocument,
    "\n  mutation ShareItem($input: ShareInput!) {\n    shareItem(input: $input) {\n      id\n      subjectType\n      subjectId\n      subjectName\n      role\n      noDownload\n      capabilities\n      expiresAt\n    }\n  }\n": typeof types.ShareItemDocument,
    "\n  mutation RevokeAccess($grantId: ID!) {\n    revokeAccess(grantId: $grantId)\n  }\n": typeof types.RevokeAccessDocument,
    "\n  mutation SetGeneralAccess($input: GeneralAccessInput!) {\n    setGeneralAccess(input: $input) {\n      level\n      role\n      noDownload\n      expiresAt\n    }\n  }\n": typeof types.SetGeneralAccessDocument,
    "\n  mutation SetInheritance($itemId: ID!, $inherit: Boolean!) {\n    setInheritance(itemId: $itemId, inherit: $inherit)\n  }\n": typeof types.SetInheritanceDocument,
    "\n  query GetSubfoldersSidebar($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n": typeof types.GetSubfoldersSidebarDocument,
    "\n  query GetEEAuthConfig($alias: String!) {\n    tenantAuthConfigByAlias(alias: $alias) {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n": typeof types.GetEeAuthConfigDocument,
    "\n  subscription DriveItemChanged {\n    driveItemChanged {\n      eventType\n      itemId\n      deletedId\n    }\n  }\n": typeof types.DriveItemChangedDocument,
    "\n  query GetMe {\n    me {\n      userId\n      tenantId\n      email\n      displayName\n      role\n      permissions\n      assignableRoles\n    }\n  }\n": typeof types.GetMeDocument,
    "\n  query AdminUsers(\n    $first: Int\n    $after: String\n    $search: String\n    $sourceId: ID\n    $status: UserStatus\n  ) {\n    adminUsers(\n      first: $first\n      after: $after\n      search: $search\n      sourceId: $sourceId\n      status: $status\n    ) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          email\n          displayName\n          role\n          disabled\n          disabledAt\n          createdAt\n          editable\n          manageable\n          source {\n            id\n            name\n            type\n            isLocal\n          }\n        }\n      }\n    }\n  }\n": typeof types.AdminUsersDocument,
    "\n  query AdminIdentitySources {\n    adminIdentitySources {\n      id\n      name\n      type\n      isLocal\n    }\n  }\n": typeof types.AdminIdentitySourcesDocument,
    "\n  mutation CreateLocalUser($input: CreateLocalUserInput!) {\n    createLocalUser(input: $input) {\n      id\n    }\n  }\n": typeof types.CreateLocalUserDocument,
    "\n  mutation UpdateLocalUser($id: ID!, $input: UpdateLocalUserInput!) {\n    updateLocalUser(id: $id, input: $input) {\n      id\n      displayName\n      role\n      manageable\n    }\n  }\n": typeof types.UpdateLocalUserDocument,
    "\n  mutation ResetLocalUserPassword($id: ID!, $password: String!) {\n    resetLocalUserPassword(id: $id, password: $password)\n  }\n": typeof types.ResetLocalUserPasswordDocument,
    "\n  mutation SetUserDisabled($id: ID!, $disabled: Boolean!) {\n    setUserDisabled(id: $id, disabled: $disabled) {\n      id\n      disabled\n      disabledAt\n    }\n  }\n": typeof types.SetUserDisabledDocument,
    "\n  query GetDrives {\n    drives {\n      id\n      name\n      driveMetadata {\n        driveType\n      }\n    }\n  }\n": typeof types.GetDrivesDocument,
    "\n  query CanCreateSharedDrive {\n    canCreateSharedDrive\n  }\n": typeof types.CanCreateSharedDriveDocument,
    "\n  mutation CreateSharedDrive($name: String!) {\n    createSharedDrive(name: $name) {\n      id\n      name\n    }\n  }\n": typeof types.CreateSharedDriveDocument,
    "\n  query GetFolderInfo($id: ID!) {\n    item(id: $id) {\n      id\n      parentId\n      name\n      type\n      myCapabilities\n      path {\n        id\n        name\n      }\n    }\n  }\n": typeof types.GetFolderInfoDocument,
    "\n  query GetFolderContents($folderId: ID!, $first: Int, $after: String) {\n    folderContents(folderId: $folderId, first: $first, after: $after) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          parentId\n          name\n          type\n          myCapabilities\n          createdAt\n          updatedAt\n          ... on File {\n            size\n            mimeType\n          }\n        }\n      }\n    }\n  }\n": typeof types.GetFolderContentsDocument,
    "\n  query GetSharedWithMe($first: Int, $after: String) {\n    sharedWithMe(first: $first, after: $after) {\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          role\n          item {\n            id\n            parentId\n            name\n            type\n            myCapabilities\n            createdAt\n            updatedAt\n            ... on File {\n              size\n              mimeType\n            }\n          }\n        }\n      }\n    }\n  }\n": typeof types.GetSharedWithMeDocument,
};
const documents: Documents = {
    "\n  query GetAuthConfig {\n    tenantAuthConfig {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n": types.GetAuthConfigDocument,
    "\n  mutation CreateFolder($parentId: ID!, $name: String!) {\n    createFolder(parentId: $parentId, name: $name) {\n      id\n      name\n    }\n  }\n": types.CreateFolderDocument,
    "\n  mutation RenameItem($id: ID!, $newName: String!) {\n    renameItem(id: $id, newName: $newName) {\n      id\n      name\n    }\n  }\n": types.RenameItemDocument,
    "\n  mutation MoveItem($id: ID!, $newParentId: ID!) {\n    moveItem(id: $id, newParentId: $newParentId) {\n      id\n      name\n      parentId\n    }\n  }\n": types.MoveItemDocument,
    "\n  mutation CopyFile($fileId: ID!, $newParentId: ID!, $newName: String!) {\n    copyFile(fileId: $fileId, newParentId: $newParentId, newName: $newName) {\n      id\n      name\n      parentId\n    }\n  }\n": types.CopyFileDocument,
    "\n  query GetDrivesForPicker {\n    drives {\n      id\n      name\n    }\n  }\n": types.GetDrivesForPickerDocument,
    "\n  query GetSubfoldersPicker($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n": types.GetSubfoldersPickerDocument,
    "\n  query ItemAccess($itemId: ID!) {\n    itemAccess(itemId: $itemId) {\n      itemId\n      inheritsPermissions\n      owner {\n        type\n        id\n        name\n        email\n        isYou\n      }\n      generalAccess {\n        level\n        role\n        noDownload\n        expiresAt\n      }\n      grants {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n      }\n      inherited {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n        inheritedFrom {\n          id\n          name\n        }\n      }\n    }\n  }\n": types.ItemAccessDocument,
    "\n  query ShareRoles($itemId: ID!) {\n    shareRoles(itemId: $itemId) {\n      role\n      label\n      description\n      capabilities\n      downloadOptional\n    }\n  }\n": types.ShareRolesDocument,
    "\n  query GeneralAccessOptions($itemId: ID!) {\n    generalAccessOptions(itemId: $itemId) {\n      level\n      label\n      blurb\n      roles\n      supportsExpiry\n    }\n  }\n": types.GeneralAccessOptionsDocument,
    "\n  query SearchDirectory($query: String!, $first: Int, $exclude: ID) {\n    searchDirectory(\n      query: $query\n      first: $first\n      excludeAccessToItemId: $exclude\n    ) {\n      type\n      id\n      name\n      email\n    }\n  }\n": types.SearchDirectoryDocument,
    "\n  mutation ShareItem($input: ShareInput!) {\n    shareItem(input: $input) {\n      id\n      subjectType\n      subjectId\n      subjectName\n      role\n      noDownload\n      capabilities\n      expiresAt\n    }\n  }\n": types.ShareItemDocument,
    "\n  mutation RevokeAccess($grantId: ID!) {\n    revokeAccess(grantId: $grantId)\n  }\n": types.RevokeAccessDocument,
    "\n  mutation SetGeneralAccess($input: GeneralAccessInput!) {\n    setGeneralAccess(input: $input) {\n      level\n      role\n      noDownload\n      expiresAt\n    }\n  }\n": types.SetGeneralAccessDocument,
    "\n  mutation SetInheritance($itemId: ID!, $inherit: Boolean!) {\n    setInheritance(itemId: $itemId, inherit: $inherit)\n  }\n": types.SetInheritanceDocument,
    "\n  query GetSubfoldersSidebar($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n": types.GetSubfoldersSidebarDocument,
    "\n  query GetEEAuthConfig($alias: String!) {\n    tenantAuthConfigByAlias(alias: $alias) {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n": types.GetEeAuthConfigDocument,
    "\n  subscription DriveItemChanged {\n    driveItemChanged {\n      eventType\n      itemId\n      deletedId\n    }\n  }\n": types.DriveItemChangedDocument,
    "\n  query GetMe {\n    me {\n      userId\n      tenantId\n      email\n      displayName\n      role\n      permissions\n      assignableRoles\n    }\n  }\n": types.GetMeDocument,
    "\n  query AdminUsers(\n    $first: Int\n    $after: String\n    $search: String\n    $sourceId: ID\n    $status: UserStatus\n  ) {\n    adminUsers(\n      first: $first\n      after: $after\n      search: $search\n      sourceId: $sourceId\n      status: $status\n    ) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          email\n          displayName\n          role\n          disabled\n          disabledAt\n          createdAt\n          editable\n          manageable\n          source {\n            id\n            name\n            type\n            isLocal\n          }\n        }\n      }\n    }\n  }\n": types.AdminUsersDocument,
    "\n  query AdminIdentitySources {\n    adminIdentitySources {\n      id\n      name\n      type\n      isLocal\n    }\n  }\n": types.AdminIdentitySourcesDocument,
    "\n  mutation CreateLocalUser($input: CreateLocalUserInput!) {\n    createLocalUser(input: $input) {\n      id\n    }\n  }\n": types.CreateLocalUserDocument,
    "\n  mutation UpdateLocalUser($id: ID!, $input: UpdateLocalUserInput!) {\n    updateLocalUser(id: $id, input: $input) {\n      id\n      displayName\n      role\n      manageable\n    }\n  }\n": types.UpdateLocalUserDocument,
    "\n  mutation ResetLocalUserPassword($id: ID!, $password: String!) {\n    resetLocalUserPassword(id: $id, password: $password)\n  }\n": types.ResetLocalUserPasswordDocument,
    "\n  mutation SetUserDisabled($id: ID!, $disabled: Boolean!) {\n    setUserDisabled(id: $id, disabled: $disabled) {\n      id\n      disabled\n      disabledAt\n    }\n  }\n": types.SetUserDisabledDocument,
    "\n  query GetDrives {\n    drives {\n      id\n      name\n      driveMetadata {\n        driveType\n      }\n    }\n  }\n": types.GetDrivesDocument,
    "\n  query CanCreateSharedDrive {\n    canCreateSharedDrive\n  }\n": types.CanCreateSharedDriveDocument,
    "\n  mutation CreateSharedDrive($name: String!) {\n    createSharedDrive(name: $name) {\n      id\n      name\n    }\n  }\n": types.CreateSharedDriveDocument,
    "\n  query GetFolderInfo($id: ID!) {\n    item(id: $id) {\n      id\n      parentId\n      name\n      type\n      myCapabilities\n      path {\n        id\n        name\n      }\n    }\n  }\n": types.GetFolderInfoDocument,
    "\n  query GetFolderContents($folderId: ID!, $first: Int, $after: String) {\n    folderContents(folderId: $folderId, first: $first, after: $after) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          parentId\n          name\n          type\n          myCapabilities\n          createdAt\n          updatedAt\n          ... on File {\n            size\n            mimeType\n          }\n        }\n      }\n    }\n  }\n": types.GetFolderContentsDocument,
    "\n  query GetSharedWithMe($first: Int, $after: String) {\n    sharedWithMe(first: $first, after: $after) {\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          role\n          item {\n            id\n            parentId\n            name\n            type\n            myCapabilities\n            createdAt\n            updatedAt\n            ... on File {\n              size\n              mimeType\n            }\n          }\n        }\n      }\n    }\n  }\n": types.GetSharedWithMeDocument,
};

/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 *
 *
 * @example
 * ```ts
 * const query = graphql(`query GetUser($id: ID!) { user(id: $id) { name } }`);
 * ```
 *
 * The query argument is unknown!
 * Please regenerate the types.
 */
export function graphql(source: string): unknown;

/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetAuthConfig {\n    tenantAuthConfig {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetAuthConfig {\n    tenantAuthConfig {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateFolder($parentId: ID!, $name: String!) {\n    createFolder(parentId: $parentId, name: $name) {\n      id\n      name\n    }\n  }\n"): (typeof documents)["\n  mutation CreateFolder($parentId: ID!, $name: String!) {\n    createFolder(parentId: $parentId, name: $name) {\n      id\n      name\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation RenameItem($id: ID!, $newName: String!) {\n    renameItem(id: $id, newName: $newName) {\n      id\n      name\n    }\n  }\n"): (typeof documents)["\n  mutation RenameItem($id: ID!, $newName: String!) {\n    renameItem(id: $id, newName: $newName) {\n      id\n      name\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation MoveItem($id: ID!, $newParentId: ID!) {\n    moveItem(id: $id, newParentId: $newParentId) {\n      id\n      name\n      parentId\n    }\n  }\n"): (typeof documents)["\n  mutation MoveItem($id: ID!, $newParentId: ID!) {\n    moveItem(id: $id, newParentId: $newParentId) {\n      id\n      name\n      parentId\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CopyFile($fileId: ID!, $newParentId: ID!, $newName: String!) {\n    copyFile(fileId: $fileId, newParentId: $newParentId, newName: $newName) {\n      id\n      name\n      parentId\n    }\n  }\n"): (typeof documents)["\n  mutation CopyFile($fileId: ID!, $newParentId: ID!, $newName: String!) {\n    copyFile(fileId: $fileId, newParentId: $newParentId, newName: $newName) {\n      id\n      name\n      parentId\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetDrivesForPicker {\n    drives {\n      id\n      name\n    }\n  }\n"): (typeof documents)["\n  query GetDrivesForPicker {\n    drives {\n      id\n      name\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetSubfoldersPicker($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetSubfoldersPicker($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query ItemAccess($itemId: ID!) {\n    itemAccess(itemId: $itemId) {\n      itemId\n      inheritsPermissions\n      owner {\n        type\n        id\n        name\n        email\n        isYou\n      }\n      generalAccess {\n        level\n        role\n        noDownload\n        expiresAt\n      }\n      grants {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n      }\n      inherited {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n        inheritedFrom {\n          id\n          name\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query ItemAccess($itemId: ID!) {\n    itemAccess(itemId: $itemId) {\n      itemId\n      inheritsPermissions\n      owner {\n        type\n        id\n        name\n        email\n        isYou\n      }\n      generalAccess {\n        level\n        role\n        noDownload\n        expiresAt\n      }\n      grants {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n      }\n      inherited {\n        id\n        subjectType\n        subjectId\n        subjectName\n        role\n        noDownload\n        capabilities\n        expiresAt\n        isYou\n        inheritedFrom {\n          id\n          name\n        }\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query ShareRoles($itemId: ID!) {\n    shareRoles(itemId: $itemId) {\n      role\n      label\n      description\n      capabilities\n      downloadOptional\n    }\n  }\n"): (typeof documents)["\n  query ShareRoles($itemId: ID!) {\n    shareRoles(itemId: $itemId) {\n      role\n      label\n      description\n      capabilities\n      downloadOptional\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GeneralAccessOptions($itemId: ID!) {\n    generalAccessOptions(itemId: $itemId) {\n      level\n      label\n      blurb\n      roles\n      supportsExpiry\n    }\n  }\n"): (typeof documents)["\n  query GeneralAccessOptions($itemId: ID!) {\n    generalAccessOptions(itemId: $itemId) {\n      level\n      label\n      blurb\n      roles\n      supportsExpiry\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query SearchDirectory($query: String!, $first: Int, $exclude: ID) {\n    searchDirectory(\n      query: $query\n      first: $first\n      excludeAccessToItemId: $exclude\n    ) {\n      type\n      id\n      name\n      email\n    }\n  }\n"): (typeof documents)["\n  query SearchDirectory($query: String!, $first: Int, $exclude: ID) {\n    searchDirectory(\n      query: $query\n      first: $first\n      excludeAccessToItemId: $exclude\n    ) {\n      type\n      id\n      name\n      email\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ShareItem($input: ShareInput!) {\n    shareItem(input: $input) {\n      id\n      subjectType\n      subjectId\n      subjectName\n      role\n      noDownload\n      capabilities\n      expiresAt\n    }\n  }\n"): (typeof documents)["\n  mutation ShareItem($input: ShareInput!) {\n    shareItem(input: $input) {\n      id\n      subjectType\n      subjectId\n      subjectName\n      role\n      noDownload\n      capabilities\n      expiresAt\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation RevokeAccess($grantId: ID!) {\n    revokeAccess(grantId: $grantId)\n  }\n"): (typeof documents)["\n  mutation RevokeAccess($grantId: ID!) {\n    revokeAccess(grantId: $grantId)\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation SetGeneralAccess($input: GeneralAccessInput!) {\n    setGeneralAccess(input: $input) {\n      level\n      role\n      noDownload\n      expiresAt\n    }\n  }\n"): (typeof documents)["\n  mutation SetGeneralAccess($input: GeneralAccessInput!) {\n    setGeneralAccess(input: $input) {\n      level\n      role\n      noDownload\n      expiresAt\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation SetInheritance($itemId: ID!, $inherit: Boolean!) {\n    setInheritance(itemId: $itemId, inherit: $inherit)\n  }\n"): (typeof documents)["\n  mutation SetInheritance($itemId: ID!, $inherit: Boolean!) {\n    setInheritance(itemId: $itemId, inherit: $inherit)\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetSubfoldersSidebar($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetSubfoldersSidebar($folderId: ID!) {\n    folderContents(folderId: $folderId, first: 100) {\n      edges {\n        node {\n          id\n          name\n          type\n        }\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetEEAuthConfig($alias: String!) {\n    tenantAuthConfigByAlias(alias: $alias) {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetEEAuthConfig($alias: String!) {\n    tenantAuthConfigByAlias(alias: $alias) {\n      tenantId\n      name\n      alias\n      defaultIdpId\n      providers {\n        id\n        name\n        type\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  subscription DriveItemChanged {\n    driveItemChanged {\n      eventType\n      itemId\n      deletedId\n    }\n  }\n"): (typeof documents)["\n  subscription DriveItemChanged {\n    driveItemChanged {\n      eventType\n      itemId\n      deletedId\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetMe {\n    me {\n      userId\n      tenantId\n      email\n      displayName\n      role\n      permissions\n      assignableRoles\n    }\n  }\n"): (typeof documents)["\n  query GetMe {\n    me {\n      userId\n      tenantId\n      email\n      displayName\n      role\n      permissions\n      assignableRoles\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AdminUsers(\n    $first: Int\n    $after: String\n    $search: String\n    $sourceId: ID\n    $status: UserStatus\n  ) {\n    adminUsers(\n      first: $first\n      after: $after\n      search: $search\n      sourceId: $sourceId\n      status: $status\n    ) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          email\n          displayName\n          role\n          disabled\n          disabledAt\n          createdAt\n          editable\n          manageable\n          source {\n            id\n            name\n            type\n            isLocal\n          }\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query AdminUsers(\n    $first: Int\n    $after: String\n    $search: String\n    $sourceId: ID\n    $status: UserStatus\n  ) {\n    adminUsers(\n      first: $first\n      after: $after\n      search: $search\n      sourceId: $sourceId\n      status: $status\n    ) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          email\n          displayName\n          role\n          disabled\n          disabledAt\n          createdAt\n          editable\n          manageable\n          source {\n            id\n            name\n            type\n            isLocal\n          }\n        }\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AdminIdentitySources {\n    adminIdentitySources {\n      id\n      name\n      type\n      isLocal\n    }\n  }\n"): (typeof documents)["\n  query AdminIdentitySources {\n    adminIdentitySources {\n      id\n      name\n      type\n      isLocal\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateLocalUser($input: CreateLocalUserInput!) {\n    createLocalUser(input: $input) {\n      id\n    }\n  }\n"): (typeof documents)["\n  mutation CreateLocalUser($input: CreateLocalUserInput!) {\n    createLocalUser(input: $input) {\n      id\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation UpdateLocalUser($id: ID!, $input: UpdateLocalUserInput!) {\n    updateLocalUser(id: $id, input: $input) {\n      id\n      displayName\n      role\n      manageable\n    }\n  }\n"): (typeof documents)["\n  mutation UpdateLocalUser($id: ID!, $input: UpdateLocalUserInput!) {\n    updateLocalUser(id: $id, input: $input) {\n      id\n      displayName\n      role\n      manageable\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ResetLocalUserPassword($id: ID!, $password: String!) {\n    resetLocalUserPassword(id: $id, password: $password)\n  }\n"): (typeof documents)["\n  mutation ResetLocalUserPassword($id: ID!, $password: String!) {\n    resetLocalUserPassword(id: $id, password: $password)\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation SetUserDisabled($id: ID!, $disabled: Boolean!) {\n    setUserDisabled(id: $id, disabled: $disabled) {\n      id\n      disabled\n      disabledAt\n    }\n  }\n"): (typeof documents)["\n  mutation SetUserDisabled($id: ID!, $disabled: Boolean!) {\n    setUserDisabled(id: $id, disabled: $disabled) {\n      id\n      disabled\n      disabledAt\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetDrives {\n    drives {\n      id\n      name\n      driveMetadata {\n        driveType\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetDrives {\n    drives {\n      id\n      name\n      driveMetadata {\n        driveType\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query CanCreateSharedDrive {\n    canCreateSharedDrive\n  }\n"): (typeof documents)["\n  query CanCreateSharedDrive {\n    canCreateSharedDrive\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateSharedDrive($name: String!) {\n    createSharedDrive(name: $name) {\n      id\n      name\n    }\n  }\n"): (typeof documents)["\n  mutation CreateSharedDrive($name: String!) {\n    createSharedDrive(name: $name) {\n      id\n      name\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetFolderInfo($id: ID!) {\n    item(id: $id) {\n      id\n      parentId\n      name\n      type\n      myCapabilities\n      path {\n        id\n        name\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetFolderInfo($id: ID!) {\n    item(id: $id) {\n      id\n      parentId\n      name\n      type\n      myCapabilities\n      path {\n        id\n        name\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetFolderContents($folderId: ID!, $first: Int, $after: String) {\n    folderContents(folderId: $folderId, first: $first, after: $after) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          parentId\n          name\n          type\n          myCapabilities\n          createdAt\n          updatedAt\n          ... on File {\n            size\n            mimeType\n          }\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetFolderContents($folderId: ID!, $first: Int, $after: String) {\n    folderContents(folderId: $folderId, first: $first, after: $after) {\n      totalCount\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          id\n          parentId\n          name\n          type\n          myCapabilities\n          createdAt\n          updatedAt\n          ... on File {\n            size\n            mimeType\n          }\n        }\n      }\n    }\n  }\n"];
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query GetSharedWithMe($first: Int, $after: String) {\n    sharedWithMe(first: $first, after: $after) {\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          role\n          item {\n            id\n            parentId\n            name\n            type\n            myCapabilities\n            createdAt\n            updatedAt\n            ... on File {\n              size\n              mimeType\n            }\n          }\n        }\n      }\n    }\n  }\n"): (typeof documents)["\n  query GetSharedWithMe($first: Int, $after: String) {\n    sharedWithMe(first: $first, after: $after) {\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n      edges {\n        cursor\n        node {\n          role\n          item {\n            id\n            parentId\n            name\n            type\n            myCapabilities\n            createdAt\n            updatedAt\n            ... on File {\n              size\n              mimeType\n            }\n          }\n        }\n      }\n    }\n  }\n"];

export function graphql(source: string) {
  return (documents as any)[source] ?? {};
}

export type DocumentType<TDocumentNode extends DocumentNode<any, any>> = TDocumentNode extends DocumentNode<  infer TType,  any>  ? TType  : never;
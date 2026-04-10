package dbrepo

// 示例代码，展示如何使用新的DAO和QueryBuilder（指针版本）
/*
// 1. 创建DAO实例
// 使用默认数据库连接（platform）
dao := NewDAO[platform.User](platform.User{})

// 使用指定数据库连接
db, _ := global.Database().Get("platform")
dao := NewDAO[platform.User](platform.User{}, WithDB(db))

// 使用指定数据库名称
dao := NewDAO[platform.User](platform.User{}, WithDBName("another_db"))

// 2. 基本CRUD操作
// 创建记录
user := &platform.User{Account: "test", Password: "123456"}
err := dao.Create(user)  // 接受指针

// 查询记录
record, err := dao.First(1)  // 返回 *platform.User
record, err := dao.FirstBy("account", "test")  // 返回 *platform.User

// 更新记录
err := dao.Save(user)  // 接受 *platform.User
err := dao.Update(1, map[string]interface{}{"nickname": "new_name"})

// 删除记录
err := dao.Delete(1)
err := dao.Remove(1) // 硬删除

// 3. 使用查询构建器
queryBuilder := NewQueryBuilder[platform.User, platformfilter.User](dao)

// 分页查询
records, total, err := queryBuilder.Paginate(0, 10, nil)  // records 是 []*platform.User

// 带过滤条件查询
filter := dbquery.New[platformfilter.User](&platformfilter.User{Account: "test"})
records, err := queryBuilder.All(filter)  // records 是 []*platform.User

// 使用查询选项
records, err := queryBuilder.Find(nil,
	WithPreload("UserRoles"),
	WithSelect("id", "account", "nickname"),
	WithOrder("created_at DESC"),
	WithLimit(10),
)  // records 是 []*platform.User

// 4. 事务操作
err := dao.Transaction(func(tx *gorm.DB) error {
	// 在事务中执行多个操作
	user1 := &platform.User{Account: "user1", Password: "123456"}
	user2 := &platform.User{Account: "user2", Password: "123456"}

	if err := tx.Create(user1).Error; err != nil {
		return err
	}
	if err := tx.Create(user2).Error; err != nil {
		return err
	}
	return nil
})

// 5. 统计和判断
count, err := dao.Count()
exists, err := dao.Exists(1)

// 6. 查找或创建
userToCreate := &platform.User{Account: "test", Password: "123456"}
record, err := dao.FirstOrCreate(
	map[string]interface{}{"account": "test"},
	userToCreate,  // 传递指针
)  // 返回 *platform.User

// 7. 批量创建
users := []*platform.User{
	{Account: "user1", Password: "123456"},
	{Account: "user2", Password: "123456"},
}
err := dao.Creates(users)  // 接受 []*platform.User
*/

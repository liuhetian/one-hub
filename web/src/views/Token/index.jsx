import { useState, useEffect, useContext, useCallback } from 'react';
import { showError, showSuccess, trims, copy, useIsAdmin, useIsRoot } from 'utils/common';

import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableContainer from '@mui/material/TableContainer';
import PerfectScrollbar from 'react-perfect-scrollbar';
import TablePagination from '@mui/material/TablePagination';
import LinearProgress from '@mui/material/LinearProgress';
import Alert from '@mui/material/Alert';
import ButtonGroup from '@mui/material/ButtonGroup';
import Toolbar from '@mui/material/Toolbar';
import Autocomplete from '@mui/material/Autocomplete';
import TextField from '@mui/material/TextField';
import Chip from '@mui/material/Chip';

import { Button, Card, Box, Stack, Container, Typography } from '@mui/material';
import TokensTableRow from './component/TableRow';
import KeywordTableHead from 'ui-component/TableHead';
import TableToolBar from 'ui-component/TableToolBar';
import { API } from 'utils/api';
import { Icon } from '@iconify/react';
import EditeModal from './component/EditModal';
import { useSelector } from 'react-redux';
import { PAGE_SIZE_OPTIONS, getPageSize, savePageSize } from 'constants';
import { useTranslation } from 'react-i18next';
import { UserContext } from 'contexts/UserContext';

export default function Token() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('id');
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('token'));
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [tokens, setTokens] = useState([]);
  const [refreshFlag, setRefreshFlag] = useState(false);
  const { loadUserGroup } = useContext(UserContext);
  const [userGroupOptions, setUserGroupOptions] = useState([]);

  const [openModal, setOpenModal] = useState(false);
  const [editTokenId, setEditTokenId] = useState(0);
  const siteInfo = useSelector((state) => state.siteInfo);
  const { userGroup } = useSelector((state) => state.account);
  const userIsAdmin = useIsAdmin();

  // 超级管理员查看其他用户token的功能
  const userIsRoot = useIsRoot();
  const [userOptions, setUserOptions] = useState([]);
  const [selectedUser, setSelectedUser] = useState(null);
  const [userSearchKeyword, setUserSearchKeyword] = useState('');
  const [loadingUsers, setLoadingUsers] = useState(false);

  const handleSort = (event, id) => {
    const isAsc = orderBy === id && order === 'asc';
    if (id !== '') {
      setOrder(isAsc ? 'desc' : 'asc');
      setOrderBy(id);
    }
  };

  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    const newRowsPerPage = parseInt(event.target.value, 10);
    setPage(0);
    setRowsPerPage(newRowsPerPage);
    savePageSize('token', newRowsPerPage);
  };

  const searchTokens = async (event) => {
    event.preventDefault();
    const formData = new FormData(event.target);
    setPage(0);
    setSearchKeyword(formData.get('keyword'));
  };

  // 获取用户列表（仅超级管理员可用）
  const fetchUsers = useCallback(async (keyword) => {
    if (!userIsRoot) return;
    setLoadingUsers(true);
    try {
      const res = await API.get('/api/user/', {
        params: {
          page: 1,
          size: 20,
          keyword: keyword || ''
        }
      });
      const { success, data } = res.data;
      if (success && data.data) {
        const options = data.data.map((user) => ({
          id: user.id,
          username: user.username,
          display_name: user.display_name || user.username
        }));
        setUserOptions(options);
      }
    } catch (error) {
      console.error(error);
    }
    setLoadingUsers(false);
  }, [userIsRoot]);

  // 初始加载用户列表
  useEffect(() => {
    if (userIsRoot) {
      fetchUsers('');
    }
  }, [userIsRoot, fetchUsers]);

  // 用户搜索（包括清空搜索时重新加载用户列表）
  useEffect(() => {
    if (userIsRoot) {
      const timer = setTimeout(() => {
        fetchUsers(userSearchKeyword);
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [userSearchKeyword, userIsRoot, fetchUsers]);

  const fetchData = async (page, rowsPerPage, keyword, order, orderBy) => {
    setSearching(true);
    keyword = trims(keyword);
    try {
      if (orderBy) {
        orderBy = order === 'desc' ? '-' + orderBy : orderBy;
      }

      // 如果是超级管理员且选择了用户，使用管理员接口
      let res;
      if (userIsRoot && selectedUser) {
        res = await API.get('/api/token/admin/', {
          params: {
            page: page + 1,
            size: rowsPerPage,
            keyword: keyword,
            order: orderBy,
            user_id: selectedUser.id
          }
        });
      } else {
        res = await API.get('/api/token/', {
          params: {
            page: page + 1,
            size: rowsPerPage,
            keyword: keyword,
            order: orderBy
          }
        });
      }

      const { success, message, data } = res.data;
      if (success) {
        setListCount(data.total_count);
        setTokens(data.data);
      } else {
        showError(message);
      }
    } catch (error) {
      console.error(error);
    }
    setSearching(false);
  };

  // 处理刷新
  const handleRefresh = async () => {
    setOrderBy('id');
    setOrder('desc');
    setRefreshFlag(!refreshFlag);
  };

  useEffect(() => {
    fetchData(page, rowsPerPage, searchKeyword, order, orderBy);
  }, [page, rowsPerPage, searchKeyword, order, orderBy, refreshFlag, selectedUser]);

  useEffect(() => {
    loadUserGroup();
  }, [loadUserGroup]);

  useEffect(() => {
    let options = [];
    Object.values(userGroup).forEach((item) => {
      options.push({ label: `${item.name} (倍率：${item.ratio})`, value: item.symbol });
    });
    setUserGroupOptions(options);
  }, [userGroup]);

  const manageToken = async (id, action, value) => {
    // 根据是否是超级管理员查看他人token来决定使用哪个接口
    const isAdminMode = userIsRoot && selectedUser;
    const url = isAdminMode ? '/api/token/admin/' : '/api/token/';
    let data = { id };
    let res;
    try {
      switch (action) {
        case 'delete':
          res = await API.delete(url + id);
          break;
        case 'status':
          res = await API.put(url + `?status_only=true`, {
            ...data,
            status: value
          });
          break;
      }
      const { success, message } = res.data;
      if (success) {
        showSuccess('操作成功完成！');
        if (action === 'delete') {
          await handleRefresh();
        }
      } else {
        showError(message);
      }

      return res.data;
    } catch (error) {
      showError(error);
    }
  };

  const handleOpenModal = (tokenId) => {
    setEditTokenId(tokenId);
    setOpenModal(true);
  };

  const handleCloseModal = () => {
    setOpenModal(false);
    setEditTokenId(0);
  };

  const handleOkModal = (status) => {
    if (status === true) {
      handleCloseModal();
      handleRefresh();
    }
  };

  return (
    <>
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={5}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h2">{t('token_index.token')}</Typography>
          <Typography variant="subtitle1" color="text.secondary">
            Token
          </Typography>
        </Stack>

        <Button
          variant="contained"
          color="primary"
          onClick={() => {
            handleOpenModal(0);
          }}
          startIcon={<Icon icon="solar:add-circle-line-duotone" />}
        >
          {t('token_index.createToken')}
        </Button>
      </Stack>
      <Stack mb={5}>
        <Alert severity="info">
          {t('token_index.replaceApiAddress1')}
          <Box
            component="span"
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              backgroundColor: 'rgba(0, 0, 0, 0.08)',
              padding: '4px 8px',
              borderRadius: '4px',
              margin: '0 4px',
              cursor: 'pointer',
              '&:hover': {
                backgroundColor: 'rgba(0, 0, 0, 0.12)'
              }
            }}
            onClick={() => copy(siteInfo.server_address, 'API地址')}
          >
            <b>{siteInfo.server_address}</b>
            <Icon icon="solar:copy-line-duotone" style={{ marginLeft: '8px', fontSize: '18px' }} />
          </Box>
          {t('token_index.replaceApiAddress2')}
        </Alert>
      </Stack>
      <Card>
        {/* 超级管理员用户选择器 */}
        {userIsRoot && (
          <Box sx={{ p: 2, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Stack direction="row" spacing={2} alignItems="center">
              <Typography variant="subtitle2" color="text.secondary" sx={{ minWidth: 100 }}>
                {t('token_index.selectUser') || '选择用户'}:
              </Typography>
              <Autocomplete
                sx={{ minWidth: 300 }}
                size="small"
                options={userOptions}
                loading={loadingUsers}
                value={selectedUser}
                onChange={(event, newValue) => {
                  setSelectedUser(newValue);
                  setPage(0);
                }}
                onInputChange={(event, newInputValue) => {
                  setUserSearchKeyword(newInputValue);
                }}
                getOptionLabel={(option) => `${option.display_name} (ID: ${option.id})`}
                isOptionEqualToValue={(option, value) => option.id === value?.id}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    placeholder={t('token_index.searchUserPlaceholder') || '搜索用户名或ID...'}
                    variant="outlined"
                  />
                )}
                renderOption={(props, option) => (
                  <li {...props} key={option.id}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <span>{option.display_name}</span>
                      <Chip label={`ID: ${option.id}`} size="small" variant="outlined" />
                      {option.username !== option.display_name && (
                        <Chip label={option.username} size="small" color="default" />
                      )}
                    </Stack>
                  </li>
                )}
                noOptionsText={t('token_index.noUserFound') || '未找到用户'}
                loadingText={t('token_index.loadingUsers') || '加载中...'}
              />
              {selectedUser && (
                <Chip
                  label={t('token_index.viewingUserTokens') || `正在查看用户 ${selectedUser.display_name} 的令牌`}
                  color="warning"
                  onDelete={() => setSelectedUser(null)}
                />
              )}
            </Stack>
          </Box>
        )}
        <Box component="form" onSubmit={searchTokens} noValidate>
          <TableToolBar placeholder={t('token_index.searchTokenName')} />
        </Box>
        <Toolbar
          sx={{
            textAlign: 'right',
            height: 50,
            display: 'flex',
            justifyContent: 'space-between',
            p: (theme) => theme.spacing(0, 1, 0, 3)
          }}
        >
          <Container maxWidth="xl">
            <ButtonGroup variant="outlined" aria-label="outlined small primary button group">
              <Button onClick={handleRefresh} startIcon={<Icon icon="solar:refresh-bold-duotone" width={18} />}>
                {t('token_index.refresh')}
              </Button>
            </ButtonGroup>
          </Container>
        </Toolbar>
        {searching && <LinearProgress />}
        <PerfectScrollbar component="div">
          <TableContainer sx={{ overflow: 'unset' }}>
            <Table sx={{ minWidth: 800 }}>
              <KeywordTableHead
                order={order}
                orderBy={orderBy}
                onRequestSort={handleSort}
                headLabel={[
                  { id: 'name', label: t('token_index.name'), disableSort: false },
                  { id: 'group', label: t('token_index.userGroup'), disableSort: false },
                  { id: 'billing_tag', label: t('token_index.billingTag'), disableSort: true, hide: !userIsAdmin },
                  { id: 'status', label: t('token_index.status'), disableSort: false },
                  { id: 'used_quota', label: t('token_index.usedQuota'), disableSort: false },
                  { id: 'remain_quota', label: t('token_index.remainingQuota'), disableSort: false },
                  { id: 'created_time', label: t('token_index.createdTime'), disableSort: false },
                  { id: 'expired_time', label: t('token_index.expiryTime'), disableSort: false },
                  { id: 'action', label: t('token_index.actions'), disableSort: true }
                ].filter(col => !col.hide)}
              />
              <TableBody>
                {tokens.map((row) => (
                  <TokensTableRow
                    item={row}
                    manageToken={manageToken}
                    key={row.id}
                    handleOpenModal={handleOpenModal}
                    setModalTokenId={setEditTokenId}
                    userGroup={userGroup}
                    userIsAdmin={userIsAdmin}
                  />
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </PerfectScrollbar>
        <TablePagination
          page={page}
          component="div"
          count={listCount}
          rowsPerPage={rowsPerPage}
          onPageChange={handleChangePage}
          rowsPerPageOptions={PAGE_SIZE_OPTIONS}
          onRowsPerPageChange={handleChangeRowsPerPage}
          showFirstButton
          showLastButton
        />
      </Card>
      <EditeModal
        open={openModal}
        onCancel={handleCloseModal}
        onOk={handleOkModal}
        tokenId={editTokenId}
        userGroupOptions={userGroupOptions}
        isAdminMode={userIsRoot && selectedUser !== null}
      />
    </>
  );
}
